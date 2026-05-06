package mcp

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

var provinceNameMap = map[rune]string{
	'京': "北京", '津': "天津", '沪': "上海", '渝': "重庆",
	'冀': "河北", '豫': "河南", '云': "云南", '辽': "辽宁", '黑': "黑龙江",
	'湘': "湖南", '皖': "安徽", '鲁': "山东", '新': "新疆", '苏': "江苏",
	'浙': "浙江", '赣': "江西", '鄂': "湖北", '桂': "广西", '甘': "甘肃",
	'晋': "山西", '蒙': "内蒙古", '陕': "陕西", '吉': "吉林", '闽': "福建",
	'贵': "贵州", '粤': "广东", '青': "青海", '藏': "西藏", '川': "四川",
	'宁': "宁夏", '琼': "海南",
}

var guangdongCityMap = map[rune]string{
	'A': "广州",
	'B': "深圳",
	'C': "珠海",
	'D': "汕头",
	'E': "佛山",
	'F': "韶关",
	'G': "湛江",
	'H': "肇庆",
	'J': "江门",
	'K': "茂名",
	'L': "惠州",
	'M': "梅州",
	'N': "汕尾",
	'P': "河源",
	'Q': "阳江",
	'R': "清远",
	'S': "东莞",
	'T': "中山",
	'U': "潮州",
	'V': "揭阳",
	'W': "云浮",
	'X': "顺德",
	'Y': "南海",
}

var (
	hongKongPlatePattern = regexp.MustCompile(`^[A-Z]{1,2}[0-9]{1,4}$`)
	macauPlatePattern    = regexp.MustCompile(`^M[A-Z][0-9]{2,4}$`)
)

// RecognizeVehiclePlate 识别车牌来源、地区和港澳车辆类型
func (s *sMcpTool) RecognizeVehiclePlate(ctx context.Context, request mcpgo.CallToolRequest) (out *mcpgo.CallToolResult, err error) {
	plateNumber := request.GetString("plateNumber", "")
	if strings.TrimSpace(plateNumber) == "" {
		err = errors.New("plateNumber is required")
		return
	}

	normalized := normalizePlateNumber(plateNumber)
	result := g.Map{
		"plateNumber": plateNumber,
		"normalized":  normalized,
		"type":        "unknown",
		"origin":      "未知",
		"confidence":  "low",
		"basis":       "未匹配到已支持的内地、粤Z跨境、香港或澳门车牌格式",
	}

	runes := []rune(normalized)
	if len(runes) >= 4 && runes[0] == '粤' && unicode.ToUpper(runes[1]) == 'Z' {
		suffix := runes[len(runes)-1]
		if suffix == '港' || suffix == '澳' {
			origin := "香港"
			plateType := "yue_z_hong_kong_cross_border"
			if suffix == '澳' {
				origin = "澳门"
				plateType = "yue_z_macau_cross_border"
			}
			result["type"] = plateType
			result["origin"] = origin
			result["province"] = "广东"
			result["city"] = "跨境车辆"
			result["isHongKongMacauVehicle"] = true
			result["confidence"] = "high"
			result["basis"] = "匹配粤Z跨境牌，且后缀为" + string(suffix)
			out = mcpgo.NewToolResultText(gjson.MustEncodeString(result))
			return
		}
	}

	if len(runes) >= 2 {
		province, ok := provinceNameMap[runes[0]]
		letter := unicode.ToUpper(runes[1])
		if ok && letter >= 'A' && letter <= 'Z' {
			result["type"] = "mainland_china"
			result["origin"] = "中国内地"
			result["province"] = province
			result["isHongKongMacauVehicle"] = false
			result["confidence"] = "medium"
			result["basis"] = "匹配内地机动车号牌省份简称和发牌机关字母"
			if runes[0] == '粤' {
				if city, cityOK := guangdongCityMap[letter]; cityOK {
					result["city"] = city
					result["confidence"] = "high"
					result["basis"] = "匹配广东省车牌字母映射，" + string(runes[0]) + string(letter) + " 表示" + city
				}
			}
			out = mcpgo.NewToolResultText(gjson.MustEncodeString(result))
			return
		}
	}

	asciiPlate := normalizeAsciiPlate(normalized)
	switch {
	case macauPlatePattern.MatchString(asciiPlate):
		result["type"] = "macau_local"
		result["origin"] = "澳门"
		result["isHongKongMacauVehicle"] = true
		result["confidence"] = "medium"
		result["basis"] = "匹配澳门本地车牌常见格式：M + 字母 + 数字"
	case hongKongPlatePattern.MatchString(asciiPlate):
		result["type"] = "hong_kong_local"
		result["origin"] = "香港"
		result["isHongKongMacauVehicle"] = true
		result["confidence"] = "medium"
		result["basis"] = "匹配香港本地车牌常见格式：1-2 个英文字母 + 1-4 位数字"
	default:
		result["isHongKongMacauVehicle"] = false
	}

	out = mcpgo.NewToolResultText(gjson.MustEncodeString(result))
	return
}

func normalizePlateNumber(plate string) string {
	plate = strings.TrimSpace(plate)
	plate = strings.ReplaceAll(plate, " ", "")
	plate = strings.ReplaceAll(plate, "-", "")
	plate = strings.ReplaceAll(plate, "·", "")
	return strings.ToUpper(plate)
}

func normalizeAsciiPlate(plate string) string {
	var builder strings.Builder
	for _, r := range plate {
		if r <= unicode.MaxASCII && (unicode.IsDigit(r) || unicode.IsLetter(r)) {
			builder.WriteRune(unicode.ToUpper(r))
		}
	}
	return builder.String()
}
