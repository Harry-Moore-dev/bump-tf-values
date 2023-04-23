package main

import (
	"context"
	"flag"
	"fmt"
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

	// check for command line flags
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set log level to debug")
	flag.Parse()

	// set log level to warning unless in debug mode
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// load env vars
	filePath := os.Getenv("INPUT_FILEPATH")
	varname := os.Getenv("INPUT_VARNAME")
	value := os.Getenv("INPUT_VALUE")
	log.Ctx(ctx).Debug().Str("filepath", filePath).Str("varname", varname).Str("value", value).Msg("env vars loaded")

	// open specified Terraform file
	file, err := os.OpenFile(filePath, os.O_RDWR, 0600)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error opening file %v", err)
	}
	defer func() {
		err = file.Close()
		if err != nil {
			log.Ctx(ctx).Err(err).Msgf("Error closing file %v", err)
		}
	}()

	hclFile, err := parseHclFile(ctx, file)
	if err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error parsing HCL file: %v", err)
		return
	}

	if err := updateLocal(ctx, hclFile, varname, value); err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error updating local: %v", err)
		return
	}

	if err := saveHCLToFile(file, ctx, hclFile); err != nil {
		log.Ctx(ctx).Err(err).Msgf("Error saving to file: %v", err)
		return
	}

	log.Ctx(ctx).Info().Msg("File updated successfully")
}

// saveHCLToFile saves HCL configuration to file.
func saveHCLToFile(file *os.File, ctx context.Context, hclFile *hclwrite.File) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek the start of file: %w", err)
	}

	if _, err := hclFile.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// parseHclFile reads and parses the content of the file as HCL format
func parseHclFile(ctx context.Context, file *os.File) (*hclwrite.File, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	content := make([]byte, info.Size())
	if _, err := file.Read(content); err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// parse the file content into HCL format
	hclFile, diags := hclwrite.ParseConfig(content, info.Name(), hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse file content: %s", diags)
	}

	return hclFile, nil
}

// find local
// modify local value in hclfile
func updateLocal(ctx context.Context, file *hclwrite.File, varname string, value string) error {
	found := false
	for _, block := range file.Body().Blocks() {
		if block.Type() == "locals" {
			local := block.Body().GetAttribute(varname)
			if local != nil {
				found = true
				block.Body().SetAttributeValue(varname, cty.StringVal(value))
				break // exit loop once variable is found and updated
			}
		}
	}
	if !found {
		return fmt.Errorf("local variable '%s' not found", varname)
	}
	return nil
}
