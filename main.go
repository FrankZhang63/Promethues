package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var nodeExporterBuildInfo = make(map[string]string)
var nodeOsInfo = make(map[string]string)
var nodeUnameInfo = make(map[string]string)
var nodeDmiInfo = make(map[string]string)
var nodeNetworkInfos []map[string]string

func main() {
	// 接收命令行参数
	ip := flag.String("ip", "", "目标IP")
	port := flag.String("p", "", "目标Port")
	flag.Parse()
	// 判断参数是否完整
	if *ip == "" || *port == "" {
		fmt.Println("参数未输入完整，请重新运行程序并添加完整参数")
		return
	}
	// 发送请求
	resp, err := SendReq(*ip, *port)
	if err != nil {
		fmt.Println("访问目标数据失败：请检查ip和port是否正确")
		return
	}
	/*
		1.node_dmi_info
		2.node_exporter_build_info
		3.node_network_info
		4.node_os_info
		5.node_uname_info
		按照顺序从前到后寻找
	*/
	//（1）提取node_dmi_info信息的子串
	tmpindex := 0
	dmiResult, dmiEndindex := CommonExtractMsg(resp, "node_dmi_info{")
	tmpindex += dmiEndindex
	//（2）提取node_exporter_build_info信息的子串
	buildResult, buildEndindex := CommonExtractMsg(resp[tmpindex:], "node_exporter_build_info{")
	tmpindex += buildEndindex
	networkStartIndex := tmpindex
	//（3）提取node_os_info信息的子串
	osResult, osEndindex := CommonExtractMsg(resp[tmpindex:], "node_os_info{")
	tmpindex += osEndindex
	// 提取node_network_info信息的子串,特殊模块
	NetworkExtractMsg(resp[networkStartIndex:], "node_network_info{")
	//（4）提取node_uname_info信息的子串
	unameResult, _ := CommonExtractMsg(resp[tmpindex:], "node_uname_info{")
	// 逐个序列化
	json.Unmarshal([]byte(dmiResult), &nodeDmiInfo)
	json.Unmarshal([]byte(buildResult), &nodeExporterBuildInfo)
	json.Unmarshal([]byte(osResult), &nodeOsInfo)
	json.Unmarshal([]byte(unameResult), &nodeUnameInfo)

	// 最终结果输出
	EndMsg := fmt.Sprintf("Prometheus Node Exporter:\n")
	EndMsg += CommonConcatStr(nodeExporterBuildInfo, "  node_exporter_build_info:\n")
	EndMsg += CommonConcatStr(nodeOsInfo, "  node_os_info:\n")
	EndMsg += CommonConcatStr(nodeUnameInfo, "  node_uname_info:\n")
	EndMsg += CommonConcatStr(nodeDmiInfo, "  node_dmi_info:\n")
	EndMsg += NetworkConcatStr(nodeNetworkInfos, "  node_network_info:\n")
	fmt.Println(EndMsg)
}

// SendReq 发送请求
func SendReq(ip string, port string) (string, error) {
	resp, err := http.Get("http://" + ip + ":" + port + "/metrics")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	return string(s), nil
}

// CommonExtractMsg 公共模块
func CommonExtractMsg(resp, findstr string) (result string, endIndex int) {
	startIndex := strings.Index(resp, findstr)
	// 找不到的情况
	if startIndex == -1 {
		return "", 0
	}
	endIndex = strings.Index(resp[startIndex:], "} 1")
	endIndex = endIndex + startIndex + 1
	// 提取子串的内容
	result = strings.ReplaceAll(resp[startIndex+len(findstr)-1:endIndex], "=", ":")
	re := regexp.MustCompile(`(\w+):([^,]+)`)
	result = re.ReplaceAllString(result, `"$1":$2`)
	return
}

// NetworkExtractMsg 单独的网络模块
func NetworkExtractMsg(resp, findstr string) {
	count := strings.Count(resp, findstr)
	nodeNetworkInfos = make([]map[string]string, count)
	//找到第一个开始位置
	startIndex := strings.Index(resp, findstr)
	for i := 0; i < count; i++ {
		//找到结束位置
		endIndex := strings.Index(resp[startIndex:], "} 1")
		//算出结束位置
		endIndex = endIndex + startIndex + 1
		// 提取子串的内容
		result := strings.ReplaceAll(resp[startIndex+len(findstr)-1:endIndex], "=", ":")
		// 把多余的部分截掉，使其可以被反序列化为对象
		result = strings.TrimLeft(result, "nfo")
		// 正则并且加引号，使其称为JSON格式
		re := regexp.MustCompile(`(\w+):([^,]+)`)
		result = re.ReplaceAllString(result, `"$1":$2`)
		// 反序列化
		err := json.Unmarshal([]byte(result), &nodeNetworkInfos[i])
		if err != nil {
			panic(err)
		}
		startIndex = endIndex
	}
}

// CommonConcatStr 公共模块拼接返回值
func CommonConcatStr(commonInfo map[string]string, tmpMsg string) (EndMsg string) {
	if len(commonInfo) == 0 {
		return ""
	}
	EndMsg += fmt.Sprintf(tmpMsg)
	for k, v := range commonInfo {
		if v != "" {
			EndMsg += fmt.Sprintf("    %s: %s\n", strings.ToLower(k), v)
		}
	}
	return
}

// NetworkConcatStr node_network_info模块拼接返回值
func NetworkConcatStr(NetworkInfos []map[string]string, tmpMsg string) (EndMsg string) {
	if len(NetworkInfos) == 0 {
		return ""
	}
	EndMsg += fmt.Sprintf(tmpMsg)
	for i := 0; i < len(NetworkInfos); i++ {
		if len(NetworkInfos[i]) == 0 {
			continue
		}
		EndMsg += fmt.Sprintf("    " + NetworkInfos[i]["device"] + "\n")
		for k, v := range NetworkInfos[i] {
			if v != "" {
				EndMsg += fmt.Sprintf("      %s: %s\n", strings.ToLower(k), v)
			}
		}
	}
	return
}
