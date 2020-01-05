# waveshareFontGenerator

This tool can convert `ttf` fonts to a `sFont` struct needed for waveshare ePaper displays.

This is a prototype but workes fine for me.

As the epaper lib support only non-proportional fonts, finding the correct width can be tricky.
You can configure the sizes with command line arguments (`go run main.go -h`).

GO 1.13 is needed to compile this tool.

## Usage example:

`go run main.go -f /usr/share/fonts/myfont.ttf > myCustomFont.h`
