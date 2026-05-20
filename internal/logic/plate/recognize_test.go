package plate

import "testing"

func TestRecognizePlateYueZHongKong(t *testing.T) {
	result := RecognizePlateFull("粤Z1234港")
	if result.RegionType != "cross_border" || result.Origin != "香港" || !result.IsHongKongMacau {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRecognizePlateYueZMacau(t *testing.T) {
	result := RecognizePlateFull("粤Z1234澳")
	if result.RegionType != "cross_border" || result.Origin != "澳门" || !result.IsHongKongMacau {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRecognizePlateGuangdongMasked(t *testing.T) {
	result := RecognizePlateFull("粤CD65***")
	if result.RegionType != "mainland" || result.Province != "广东" || result.City != "珠海" || result.IsHongKongMacau {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRecognizePlateHongKongLocal(t *testing.T) {
	result := RecognizePlateFull("AB1234")
	if result.RegionType != "hong_kong" || result.Origin != "香港" || !result.IsHongKongMacau {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRecognizePlateMacauLocal(t *testing.T) {
	result := RecognizePlateFull("MA1234")
	if result.RegionType != "macau" || result.Origin != "澳门" || !result.IsHongKongMacau {
		t.Fatalf("unexpected result: %+v", result)
	}
}
