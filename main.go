package main

import (
	"context"
	"flag"
	"io"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

func main() {
	// initiate logging
	logger := log.With().Logger()
	ctx := logger.WithContext(context.Background())

	//check for command line flags
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set log level to debug")
	flag.Parse()

	//Set log level to warning unless in debug mode
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	//load env vars
	filePath := os.Getenv("INPUT_FILEPATH")
	varname := os.Getenv("INPUT_VARNAME")
	value := os.Getenv("INPUT_VALUE")
	log.Ctx(ctx).Debug().Str("filepath", filePath).Str("varname", varname).Str("value", value).Msg("env vars loaded")

	file, err := openFile(filePath)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error opening file %v", err)
	}

	hclFile, err := parseHclFile(ctx, file)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error parsing HCL file %v", err)
	}

	updateLocal(ctx, hclFile, varname, value)

	saveHcl(file, ctx, hclFile)

	file.Close()
}

// save hcl configuration to file
func saveHcl(file *os.File, ctx context.Context, hclFile *hclwrite.File) {
	// move pointer to start of file
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error seeking start of file %v", err)
	}

	_, err = hclFile.WriteTo(file)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error writing to file %v", err)
	}
}

// open file from path
func openFile(filepath string) (*os.File, error) {
	file, err := os.OpenFile(filepath, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// load and parse file
func parseHclFile(ctx context.Context, file *os.File) (*hclwrite.File, error) {

	// Get the file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// Initialize the content slice with the file size
	content := make([]byte, fileSize)

	bytes, err := file.Read(content)
	log.Ctx(ctx).Debug().Msgf("Number of bytes loaded from hcl file %d", bytes)
	if err != nil {
		return nil, err
	}

	// Parse the file contents as HCL
	hclFile, diags := hclwrite.ParseConfig(content, file.Name(), hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}

	// Return the parsed HCL file
	return hclFile, nil
}

// find local
// modify local value in hclfile
func updateLocal(ctx context.Context, file *hclwrite.File, varname string, value string) {
	found := false
	for _, block := range file.Body().Blocks() {
		if block.Type() == "locals" {
			local := block.Body().GetAttribute(varname)
			if local != nil {
				found = true
				block.Body().SetAttributeValue(varname, cty.StringVal(value))
			}
		}
	}
	if !found {
		log.Ctx(ctx).Error().Msgf("Local '%s' not found", varname)
	}
}
