package imgx

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestPosterFromFanartRightHalfJPEG(t *testing.T) {
	// 构造一个“左黑右白”的 fanart，验证裁切确实取右半边。
	const (
		w = 200
		h = 100
	)
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				src.Set(x, y, color.RGBA{0, 0, 0, 255})
			} else {
				src.Set(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("encode fanart jpeg 失败：%v", err)
	}

	out, err := PosterFromFanartRightHalfJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("PosterFromFanartRightHalfJPEG 失败：%v", err)
	}

	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode poster jpeg 失败：%v", err)
	}
	gb := got.Bounds()
	if gb.Dx() != w/2 || gb.Dy() != h {
		t.Fatalf("尺寸不符合预期：got=%dx%d want=%dx%d", gb.Dx(), gb.Dy(), w/2, h)
	}

	// 取中心点像素，应接近白色（JPEG 有损，允许一定偏差）。
	c := color.RGBAModel.Convert(got.At(gb.Min.X+gb.Dx()/2, gb.Min.Y+gb.Dy()/2)).(color.RGBA)
	if c.R < 200 || c.G < 200 || c.B < 200 {
		t.Fatalf("裁切区域不符合预期：中心像素=%v（期望接近白色）", c)
	}
}

func TestPosterFromFanartRightHalfJPEG_Empty(t *testing.T) {
	if _, err := PosterFromFanartRightHalfJPEG(nil); err == nil {
		t.Fatalf("期望空输入返回错误")
	}
}
