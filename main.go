package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io/ioutil"
	"log"
	"os"

	"github.com/icza/bitio"
	flags "github.com/jessevdk/go-flags"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

var conf struct {
	Width   int            `short:"w" long:"width"   description:"font width in bytes"    default:"2"`
	Height  int            `short:"h" long:"height"  description:"font height in lines"   default:"24"`
	PPEM    int            `short:"s" long:"ppem"    description:"font size"              default:"20"`
	Xoffset int            `short:"x" long:"xoffset" description:"x offset for the runes" default:"0"`
	Yoffset int            `short:"y" long:"yoffset" description:"y offset for the runes" default:"18"`
	Font    flags.Filename `short:"f" long:"font"    description:"path to font file"      required:"true"`
	Debug   bool           `short:"d" long:"debug"   description:"display some debug information"`
}

var parser = flags.NewParser(&conf, flags.Default)

func main() {
	args, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	if len(args) > 0 {
		log.Fatal("do not provide additional parameters")
	}
	// Read the font data.
	fontBytes, err := ioutil.ReadFile(string(conf.Font))
	if err != nil {
		log.Println(err)
		return
	}
	f, err := sfnt.Parse(fontBytes)
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}

	if conf.Debug {
		i, err := f.Metrics(nil, fixed.I(conf.PPEM), font.HintingFull)
		if err != nil {
			log.Fatalf("could not get font metrics: %v", err)
		}
		log.Println("font metrics:")
		log.Printf("  Height:     %s\n", i.Height)
		log.Printf("  CapHeight:  %s\n", i.CapHeight)
		log.Printf("  Ascent:     %s\n", i.Ascent)
		log.Printf("  CaretSlope: %s\n", i.CaretSlope)
		log.Printf("  Descent:    %s\n", i.Descent)
		log.Printf("  XHeight:    %s\n", i.XHeight)

		log.Println("draw window:")
		log.Println("  width:  ", conf.Width)
		log.Println("  height: ", conf.Height)
		log.Println("  Xoffset:", conf.Xoffset)
		log.Println("  Yoffset:", conf.Yoffset)
	}

	fmt.Println(`#include "fonts.h"
#if defined(__AVR__) || defined(ARDUINO_ARCH_SAMD)
#include <avr/pgmspace.h>
#elif defined(ESP8266) || defined(ESP32)
#include <pgmspace.h>
#endif

const uint8_t FontCustom_Table [] PROGMEM =
{
`)
	width := conf.Width * 8
	height := conf.Height
	for i := 32; i <= 126; i++ { // only printable chars
		v := rune(i)
		x, err := f.GlyphIndex(nil, v)
		if err != nil {
			log.Fatalf("GlyphIndex: %v", err)
		}
		if x == 0 {
			log.Fatalf("GlyphIndex: no glyph index found for the rune '", v, "'")
		}

		originX := float32(conf.Xoffset)
		originY := float32(conf.Yoffset)

		segments, err := f.LoadGlyph(nil, x, fixed.I(conf.PPEM), nil)
		if err != nil {
			log.Fatalf("LoadGlyph: %v", err)
		}
		r := vector.NewRasterizer(width, height)
		r.DrawOp = draw.Src
		for _, seg := range segments {
			// The divisions by 64 below is because the seg.Args values have type
			// fixed.Int26_6, a 26.6 fixed point number, and 1<<6 == 64.
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				r.MoveTo(
					originX+float32(seg.Args[0].X)/64,
					originY+float32(seg.Args[0].Y)/64,
				)
			case sfnt.SegmentOpLineTo:
				r.LineTo(
					originX+float32(seg.Args[0].X)/64,
					originY+float32(seg.Args[0].Y)/64,
				)
			case sfnt.SegmentOpQuadTo:
				r.QuadTo(
					originX+float32(seg.Args[0].X)/64,
					originY+float32(seg.Args[0].Y)/64,
					originX+float32(seg.Args[1].X)/64,
					originY+float32(seg.Args[1].Y)/64,
				)
			case sfnt.SegmentOpCubeTo:
				r.CubeTo(
					originX+float32(seg.Args[0].X)/64,
					originY+float32(seg.Args[0].Y)/64,
					originX+float32(seg.Args[1].X)/64,
					originY+float32(seg.Args[1].Y)/64,
					originX+float32(seg.Args[2].X)/64,
					originY+float32(seg.Args[2].Y)/64,
				)
			default:
				log.Fatal("OP: ", seg.Op)
			}
		}
		dst := image.NewAlpha(image.Rect(0, 0, width, height))
		r.Draw(dst, dst.Bounds(), image.Opaque, image.Point{})
		fmt.Printf("  // %c %d\n", v, v)
		for y := 0; y < height; y++ {
			b := &bytes.Buffer{}
			w := bitio.NewWriter(b)
			tmp := ""
			for x := 0; x < width; x++ {
				a := dst.AlphaAt(x, y).A
				if a < 64 {
					w.WriteBits(0, 1)
					tmp += "."
				} else {
					w.WriteBits(1, 1)
					tmp += "#"
				}
			}
			w.Close()
			fmt.Printf("  ")
			for _, o := range b.Bytes() {
				fmt.Printf("0x%.2X, ", o)
			}
			fmt.Printf(" // %s", tmp)
			fmt.Println()
		}
	}
	fmt.Printf(`};`)
	fmt.Printf("\n\n/* Based on font %s */\n", string(conf.Font))
	fmt.Printf(`sFONT FontCustom = {
  FontCustom_Table,
  %d, /* Width */
  %d, /* Height */
};
`, width, height)
}
