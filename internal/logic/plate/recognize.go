package plate

import (
	"regexp"
	"strings"
	"unicode"
)

var MainlandProvinceMap = map[rune]string{
	'京': "北京",
	'津': "天津",
	'沪': "上海",
	'渝': "重庆",
	'冀': "河北",
	'豫': "河南",
	'云': "云南",
	'辽': "辽宁",
	'黑': "黑龙江",
	'湘': "湖南",
	'皖': "安徽",
	'鲁': "山东",
	'新': "新疆",
	'苏': "江苏",
	'浙': "浙江",
	'赣': "江西",
	'鄂': "湖北",
	'桂': "广西",
	'甘': "甘肃",
	'晋': "山西",
	'蒙': "内蒙古",
	'陕': "陕西",
	'吉': "吉林",
	'闽': "福建",
	'贵': "贵州",
	'粤': "广东",
	'青': "青海",
	'藏': "西藏",
	'川': "四川",
	'宁': "宁夏",
	'琼': "海南",
}

var GuangdongCityMap = map[rune]string{
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
	'Z': "港澳跨境",
}

var (
	HongKongLocalPlatePattern = regexp.MustCompile(`^[A-Z]{1,2}[0-9]{1,4}$`)
	MacauLocalPlatePattern    = regexp.MustCompile(`^M[A-Z][0-9]{2,4}$`)
)

func RecognizePlate(plate string) (normalized, origin, regionType string, isHKMacau, isProvinceInside bool, province, city, plateType, confidence, basis string) {
	norm := NormalizePlate(plate)
	result := recognizePlateInner(plate, norm)
	return result.Normalized, result.Origin, result.RegionType, result.IsHongKongMacau, result.IsProvinceInside, result.Province, result.City, result.PlateType, result.Confidence, result.Basis
}

type PlateResult struct {
	Plate            string
	Normalized       string
	Origin           string
	RegionType       string
	IsHongKongMacau  bool
	IsProvinceInside bool
	Province         string
	City             string
	PlateType        string
	Confidence       string
	Basis            string
}

func RecognizePlateFull(plate string) PlateResult {
	norm := NormalizePlate(plate)
	return recognizePlateInner(plate, norm)
}

func recognizePlateInner(plate, normalized string) PlateResult {
	result := PlateResult{
		Plate:            plate,
		Normalized:       normalized,
		Origin:           "未知",
		RegionType:      "unknown",
		IsHongKongMacau:  false,
		IsProvinceInside: false,
		PlateType:       "unknown",
		Confidence:      "low",
		Basis:           "未匹配到已支持的内地、粤Z跨境、香港、澳门、军车或使领馆车牌格式",
	}
	if normalized == "" {
		result.Basis = "车牌为空"
		return result
	}

	runes := []rune(normalized)

	if len(runes) >= 4 && runes[0] == 'W' && runes[1] == 'J' {
		result.Origin = "中国内地"
		result.RegionType = "mainland"
		result.PlateType = "military_wj"
		result.Confidence = "medium"
		result.Basis = "匹配武警部队号牌：以WJ开头"
		if len(runes) >= 3 {
			if province, ok := MainlandProvinceMap[runes[2]]; ok {
				result.Province = province
			}
		}
		result.IsProvinceInside = len(runes) >= 3 && runes[2] == '粤'
		return result
	}

	if len(runes) >= 2 && (runes[0] == '使' || runes[0] == '领') {
		result.Origin = "中国内地"
		result.RegionType = "mainland"
		result.PlateType = "diplomatic"
		result.Confidence = "medium"
		if runes[0] == '使' {
			result.Basis = "匹配使馆号牌：以'使'开头"
		} else {
			result.Basis = "匹配领馆号牌：以'领'开头"
		}
		return result
	}

	if len(runes) >= 4 && runes[0] == '粤' && unicode.ToUpper(runes[1]) == 'Z' {
		suffix := runes[len(runes)-1]
		if suffix == '港' || suffix == '澳' {
			result.RegionType = "cross_border"
			result.IsHongKongMacau = true
			result.Province = "广东"
			result.City = "港澳跨境"
			result.Confidence = "high"
			if suffix == '港' {
				result.Origin = "香港"
				result.PlateType = "yue_z_hong_kong_cross_border"
				result.Basis = "匹配粤Z跨境牌，后缀为港"
			} else {
				result.Origin = "澳门"
				result.PlateType = "yue_z_macau_cross_border"
				result.Basis = "匹配粤Z跨境牌，后缀为澳"
			}
			return result
		}
	}

	if len(runes) >= 2 {
		if province, ok := MainlandProvinceMap[runes[0]]; ok {
			letter := unicode.ToUpper(runes[1])
			if letter >= 'A' && letter <= 'Z' {
				result.Origin = "中国内地"
				result.RegionType = "mainland"
				result.PlateType = "mainland_china"
				result.Province = province
				result.Confidence = "medium"
				result.Basis = "匹配内地机动车号牌省份简称和发牌机关字母"
				if len(runes) == 8 && (unicode.ToUpper(runes[2]) == 'D' || unicode.ToUpper(runes[2]) == 'F') {
					result.PlateType = "mainland_new_energy"
					result.Confidence = "high"
					result.Basis = "匹配内地新能源号牌：8位，第3位为D/F"
				}
				if runes[0] == '粤' {
					if city, cityOK := GuangdongCityMap[letter]; cityOK {
						result.City = city
						if result.Confidence != "high" {
							result.Confidence = "high"
						}
						result.Basis = "匹配广东省车牌字母映射，" + string(runes[0]) + string(letter) + " 表示" + city
					}
				}
				result.IsProvinceInside = runes[0] == '粤'
				return result
			}
		}
	}

	asciiPlate := normalizeAsciiPlate(normalized)
	switch {
	case MacauLocalPlatePattern.MatchString(asciiPlate):
		result.Origin = "澳门"
		result.RegionType = "macau"
		result.IsHongKongMacau = true
		result.PlateType = "macau_local"
		result.Confidence = "medium"
		result.Basis = "匹配澳门本地车牌常见格式：M + 字母 + 数字"
	case HongKongLocalPlatePattern.MatchString(asciiPlate):
		result.Origin = "香港"
		result.RegionType = "hong_kong"
		result.IsHongKongMacau = true
		result.PlateType = "hong_kong_local"
		result.Confidence = "medium"
		result.Basis = "匹配香港本地车牌常见格式：1-2 个英文字母 + 1-4 位数字"
	}
	return result
}

func NormalizePlate(plate string) string {
	plate = strings.TrimSpace(plate)
	plate = strings.ReplaceAll(plate, " ", "")
	plate = strings.ReplaceAll(plate, "-", "")
	plate = strings.ReplaceAll(plate, "路", "")
	plate = strings.TrimRight(plate, "警学")
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
