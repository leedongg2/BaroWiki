package tool

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/png"
	"math/big"
)

// ============================================================
//  바로 캡차 그림
//
//  글꼴 파일을 들여오지 않는다. 쓰는 글자가 스물여섯 자뿐이라
//  5×7 점판을 코드 안에 그대로 둔다. (딸린 꾸러미가 늘지 않게)
//
//  글자마다 조금씩 기울이고 흔들어 놓고, 줄과 점을 뿌린다.
//  사람 눈에는 읽히되 곧바로 글자로 오려 내기는 성가시게 하는 정도다.
// ============================================================

// baro_captcha_img_w : 그림 너비
const baro_captcha_img_w = 220

// baro_captcha_img_h : 그림 높이
const baro_captcha_img_h = 70

// baro_captcha_font : 5×7 점판. 한 줄이 한 가로줄이고, 1 인 자리가 찍힌다.
var baro_captcha_font = map[byte][7]string{
	'3': {"11111", "00010", "00100", "00010", "00001", "10001", "01110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'6': {"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'C': {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01110", "10001", "10000", "10111", "10001", "10001", "01111"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'J': {"00111", "00010", "00010", "00010", "00010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "11011", "10001"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
}

// baro_captcha_rand : 0 이상 max 미만의 수 하나
func baro_captcha_rand(max int) int {
	if max <= 0 {
		return 0
	}

	num, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}

	return int(num.Int64())
}

// Baro_captcha_png : 답 글자를 그림으로 그린다
func Baro_captcha_png(answer string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, baro_captcha_img_w, baro_captcha_img_h))

	// 바탕
	back := color.RGBA{R: 246, G: 247, B: 250, A: 255}
	for y := 0; y < baro_captcha_img_h; y++ {
		for x := 0; x < baro_captcha_img_w; x++ {
			img.Set(x, y, back)
		}
	}

	baro_captcha_noise(img, 420)
	baro_captcha_lines(img, 4)

	// 글자
	count := len(answer)
	if count == 0 {
		return baro_captcha_encode(img)
	}

	step := baro_captcha_img_w / (count + 1)

	for i := 0; i < count; i++ {
		glyph, ok := baro_captcha_font[answer[i]]
		if !ok {
			continue
		}

		scale := 6 + baro_captcha_rand(2)
		left := step*(i+1) - scale*2 + baro_captcha_rand(7) - 3
		top := (baro_captcha_img_h-scale*7)/2 + baro_captcha_rand(9) - 4
		slant := baro_captcha_rand(3) - 1

		ink := color.RGBA{
			R: uint8(20 + baro_captcha_rand(60)),
			G: uint8(20 + baro_captcha_rand(60)),
			B: uint8(70 + baro_captcha_rand(90)),
			A: 255,
		}

		baro_captcha_glyph(img, glyph, left, top, scale, slant, ink)
	}

	baro_captcha_lines(img, 3)
	baro_captcha_noise(img, 260)

	return baro_captcha_encode(img)
}

// baro_captcha_glyph : 점판 하나를 키워서 찍는다
//
//	slant 는 한 줄 내려갈 때마다 옆으로 밀리는 칸 수다. (글자가 기울어 보이게)
func baro_captcha_glyph(img *image.RGBA, glyph [7]string, left int, top int, scale int, slant int, ink color.RGBA) {
	for row := 0; row < 7; row++ {
		line := glyph[row]

		for col := 0; col < len(line); col++ {
			if line[col] != '1' {
				continue
			}

			x0 := left + col*scale + slant*(3-row)
			y0 := top + row*scale

			for y := y0; y < y0+scale; y++ {
				for x := x0; x < x0+scale; x++ {
					if x < 0 || y < 0 || x >= baro_captcha_img_w || y >= baro_captcha_img_h {
						continue
					}

					img.Set(x, y, ink)
				}
			}
		}
	}
}

// baro_captcha_noise : 점 뿌리기
func baro_captcha_noise(img *image.RGBA, num int) {
	for i := 0; i < num; i++ {
		x := baro_captcha_rand(baro_captcha_img_w)
		y := baro_captcha_rand(baro_captcha_img_h)
		gray := uint8(140 + baro_captcha_rand(90))

		img.Set(x, y, color.RGBA{R: gray, G: gray, B: uint8(int(gray)*9/10 + 10), A: 255})
	}
}

// baro_captcha_lines : 가로지르는 줄 긋기
func baro_captcha_lines(img *image.RGBA, num int) {
	for i := 0; i < num; i++ {
		x0 := baro_captcha_rand(baro_captcha_img_w)
		y0 := baro_captcha_rand(baro_captcha_img_h)
		x1 := baro_captcha_rand(baro_captcha_img_w)
		y1 := baro_captcha_rand(baro_captcha_img_h)

		ink := color.RGBA{
			R: uint8(120 + baro_captcha_rand(90)),
			G: uint8(120 + baro_captcha_rand(90)),
			B: uint8(140 + baro_captcha_rand(90)),
			A: 255,
		}

		baro_captcha_line(img, x0, y0, x1, y1, ink)
	}
}

// baro_captcha_line : 두 점을 잇는 줄 하나 (브레젠험)
func baro_captcha_line(img *image.RGBA, x0 int, y0 int, x1 int, y1 int, ink color.RGBA) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}

	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}

	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}

	err := dx - dy

	for step := 0; step < baro_captcha_img_w*2; step++ {
		if x0 >= 0 && y0 >= 0 && x0 < baro_captcha_img_w && y0 < baro_captcha_img_h {
			img.Set(x0, y0, ink)
		}

		if x0 == x1 && y0 == y1 {
			return
		}

		err2 := err * 2
		if err2 > -dy {
			err -= dy
			x0 += sx
		}
		if err2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// baro_captcha_encode : PNG 로 묶기
func baro_captcha_encode(img *image.RGBA) []byte {
	buf := &bytes.Buffer{}
	if png.Encode(buf, img) != nil {
		return nil
	}

	return buf.Bytes()
}
