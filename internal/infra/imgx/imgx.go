package imgx

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // 注册 PNG 解码器（输入不一定总是 jpeg）
)

// PosterFromFanartRightHalfJPEG 把 fanart 图片裁切为“右半边”，并编码为 JPEG（用于 poster.jpg）。
//
// 约束：
// - 输入允许是 JPEG/PNG（依赖标准库解码器）
// - 输出固定为 JPEG
// - 裁切规则：保留原高度，宽度取右半边（从 w/2 到 w）
func PosterFromFanartRightHalfJPEG(fanart []byte) ([]byte, error) {
	if len(fanart) == 0 {
		return nil, errors.New("fanart 为空")
	}

	img, _, err := image.Decode(bytes.NewReader(fanart))
	if err != nil {
		return nil, err
	}

	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("图片尺寸无效")
	}

	// 右半边：x 从 w/2 到 w，y 全保留。
	x0 := b.Min.X + b.Dx()/2
	srcRect := image.Rect(x0, b.Min.Y, b.Max.X, b.Max.Y)

	dst := image.NewRGBA(image.Rect(0, 0, srcRect.Dx(), srcRect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, srcRect.Min, draw.Src)

	var out bytes.Buffer
	// 质量：不需要太“讲究”，但要稳定可用；95 在体积与质量之间比较均衡。
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
