package route

import "testing"

// TestIpuserBand : 아이피가 속한 대역을 제대로 뽑는가
func TestIpuserBand(t *testing.T) {
	cases := []struct {
		ip   string
		want []string
	}{
		{"1.2.3.4", []string{"1.2.3.0/24", "1.2.0.0/16"}},
		{"192.168.0.1", []string{"192.168.0.0/24", "192.168.0.0/16"}},
		{"255.255.255.255", []string{"255.255.255.0/24", "255.255.0.0/16"}},
		{"0.0.0.0", []string{"0.0.0.0/24", "0.0.0.0/16"}},
	}

	for _, one := range cases {
		got := ipuser_band(one.ip)
		if len(got) != len(one.want) {
			t.Fatalf("%s : %v 를 바랐는데 %v", one.ip, one.want, got)
		}
		for index := range got {
			if got[index] != one.want[index] {
				t.Fatalf("%s : %v 를 바랐는데 %v", one.ip, one.want, got)
			}
		}
	}

	// IPv6 는 /64 하나
	if got := ipuser_band("2001:db8::1"); len(got) != 1 {
		t.Fatalf("IPv6 대역을 뽑지 못했다 : %v", got)
	}

	// 아이피가 아니면 아무것도 안 나온다
	for _, one := range []string{"", "사람이름", "999.999.999.999", "1.2.3", "1.2.3.4/24"} {
		if got := ipuser_band(one); got != nil {
			t.Fatalf("%q 는 아이피가 아닌데 대역이 나왔다 : %v", one, got)
		}
	}
}

// TestIpuserBandPrefix : 대역에서 like 검색용 앞머리를 뽑는가
func TestIpuserBandPrefix(t *testing.T) {
	cases := map[string]string{
		"1.2.3.0/24": "1.2.3.",
		"1.2.0.0/16": "1.2.",
		"10.0.0.0/8": "10.",
		"1.2.3.0/25": "1.2.3.",
		"1.2.3.4/32": "1.2.3.",
		"1.2.0.0/12": "1.",
		"잘못된 값":      "",
		"":           "",
		"1.2.3.4":    "",
	}

	for input, want := range cases {
		if got := ipuser_band_prefix(input); got != want {
			t.Fatalf("%q : %q 를 바랐는데 %q", input, want, got)
		}
	}

	// 앞머리에 LIKE 와일드카드가 섞이면 안 된다
	for _, one := range []string{"1.2.3.0/24", "1.2.0.0/16", "10.0.0.0/8"} {
		prefix := ipuser_band_prefix(one)
		for _, bad := range []rune{'%', '_', '\''} {
			for _, c := range prefix {
				if c == bad {
					t.Fatalf("%q 의 앞머리 %q 에 LIKE 특수문자가 들어 있다", one, prefix)
				}
			}
		}
	}
}
