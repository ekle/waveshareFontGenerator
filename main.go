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
	Width        int            `short:"w" long:"width" description:"font width in bytes" default:"2"`
	ReduceHeight int            `short:"r" long:"reducedHeight" description:"cut off the font height from the bottom" default:"-1"`
	PPEM         int            `short:"s" long:"ppem" description:"font size" default:"24"`
	Font         flags.Filename `short:"f" long:"font" description:"path to font file" required:"true"`
}

var parser = flags.NewParser(&conf, flags.Default)

func main() {
	args, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			//log.Fatal(err)
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
	fmt.Println(`#include "fonts.h"
#if defined(__AVR__) || defined(ARDUINO_ARCH_SAMD)
#include <avr/pgmspace.h>
#elif defined(ESP8266) || defined(ESP32)
#include <pgmspace.h>
#endif

const uint8_t FontCustom_Table [] PROGMEM =
{
`)
	var widthP int
	var heightP int
	for i := 32; i <= 126; i++ { // only printable chars
		v := rune(i)
		var b sfnt.Buffer
		x, err := f.GlyphIndex(&b, v)
		if err != nil {
			log.Fatalf("GlyphIndex: %v", err)
		}
		if x == 0 {
			log.Fatalf("GlyphIndex: no glyph index found for the rune '", v, "'")
		}
		i, err := f.Metrics(&b, fixed.I(conf.PPEM), font.HintingFull)
		if err != nil {
			log.Fatalf("could not get font metrics: %v", err)
		}
		/*
			fmt.Printf("Height:     %s\n", i.Height)
			fmt.Printf("CapHeight:  %s\n", i.CapHeight)
			fmt.Printf("Ascent:     %s\n", i.Ascent)
			fmt.Printf("CaretSlope: %s\n", i.CaretSlope)
			fmt.Printf("Descent:    %s\n", i.Descent)
			fmt.Printf("XHeight:    %s\n", i.XHeight)
		*/
		width := conf.Width * 8
		height := i.Height.Ceil() //- conf.ReduceHeight
		if conf.ReduceHeight >= 0 {
			height -= conf.ReduceHeight
		} else {
			// usually a good default
			height = height * 3 / 4
			height += 1
		}
		originX := float32(0)
		originY := float32(i.CapHeight.Ceil()*-1) + 1
		widthP = width
		heightP = height

		segments, err := f.LoadGlyph(&b, x, fixed.I(conf.PPEM), nil)
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
`, widthP, heightP)
}
