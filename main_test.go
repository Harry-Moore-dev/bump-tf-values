package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestUpdateLocalE2E(t *testing.T) {
	const happyTestFilePath = "testing/testhcl.tf"

	// create test file
	file, err := os.CreateTemp("", "testhcl.tf")
	assert.NoError(t, err, "Error creating test file")
	defer os.Remove(file.Name()) // delete the file after the test finishes

	_, err = file.WriteString(`locals {
		# pin the target versions of the code
		other_code_version = "3.3.3.3"
		code_version       = "1.1.1.1"
	  }

	  output "test_version_string" {
		value = var.other_code_version
	  }

	  output "test_version_number" {
		value = var.code_version
	  }
`)
	assert.NoError(t, err, "Error writing to test file")

	file.Close()

	// set environment variables
	os.Setenv("INPUT_FILEPATH", file.Name())
	os.Setenv("INPUT_VARNAME", "code_version")
	os.Setenv("INPUT_VALUE", "v2.55.4")

	// run e2e
	main()

	// load and check modified file matches expected file
	fileData, err := os.ReadFile(file.Name())
	assert.NoError(t, err, "Error reading test file")

	happyTestFile, err := os.ReadFile(happyTestFilePath)
	assert.NoError(t, err, "Error reading updated test file")
	assert.Equal(t, string(happyTestFile), string(fileData), "Test file and expected file don't match")
}

func TestUpdateHclFileWithFileErrorE2E(t *testing.T) {

	// create a logger
	logger := log.With().Logger()
	ctx := logger.WithContext(context.Background())

	// create test file
	file, err := os.CreateTemp("", "testhcl.tf")
	assert.NoError(t, err, "Error creating test file")
	defer os.Remove(file.Name()) // delete the file after the test finishes

	// add data with invalid syntax
	_, err = file.WriteString(`locals {
		# pin the target versions of the code
		other_code_version = "3.3.3.3"
		code_version       = "1.1.1.1"
	  }

	  output "test_version_string"
		value = var.other_code_version


	  output "test_version_number"
		value = var.code_version
	  }
`)
	assert.NoError(t, err, "Error writing to test file")

	file.Close()

	// test
	err = updateHclFile(ctx, file.Name(), "code_version", "v2.55.4")

	// check if an error was logged
	assert.ErrorContains(t, err, "failed to parse HCL file", "Expected an error parsing HCL file")
}

func TestUpdateHclFileWithLocalErrorE2E(t *testing.T) {

	// create a logger
	logger := log.With().Logger()
	ctx := logger.WithContext(context.Background())

	// create test file
	file, err := os.CreateTemp("", "testhcl.tf")
	assert.NoError(t, err, "Error creating test file")
	defer os.Remove(file.Name()) // delete the file after the test finishes

	// add data with missing local
	_, err = file.WriteString(`locals {
		# pin the target versions of the code
		other_code_version = "3.3.3.3"

	  }

	  output "test_version_string" {
		value = var.other_code_version
	  }

	  output "test_version_number" {
		value = var.code_version
	  }
`)
	assert.NoError(t, err, "Error writing to test file")

	file.Close()

	// test
	err = updateHclFile(ctx, file.Name(), "code_version", "v2.55.4")

	// check if an error was logged
	assert.ErrorContains(t, err, "failed to update local", "Expected an error parsing HCL file")
}

func TestUpdateHclFileWithSaveErrorE2E(t *testing.T) {

	// create a logger
	logger := log.With().Logger()
	ctx := logger.WithContext(context.Background())

	// create test file
	file, err := os.CreateTemp("", "testhcl.tf")
	assert.NoError(t, err, "Error creating test file")
	defer os.Remove(file.Name()) // delete the file after the test finishes

	// add data with missing local
	_, err = file.WriteString(`locals {
		# pin the target versions of the code
		other_code_version = "3.3.3.3"
		code_version       = "1.1.1.1"
	  }

	  output "test_version_string" {
		value = var.other_code_version
	  }

	  output "test_version_number" {
		value = var.code_version
	  }
`)
	assert.NoError(t, err, "Error writing to test file")

	file.Close()

	// make file readonly before attempting to write to it
	err = os.Chmod(file.Name(), 0444)
	assert.NoError(t, err, "Unable to set file as readonly")

	// test
	err = updateHclFile(ctx, file.Name(), "code_version", "v2.55.4")

	// check if an error was logged
	assert.Error(t, err, "Expected an error parsing HCL file")
}

func TestUpdateLocalNotFound(t *testing.T) {

	// Create a logger that writes to a buffer
	buf := bytes.Buffer{}
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	ctx := logger.WithContext(context.Background())

	// create a new HCL file with no locals block
	file := hclwrite.NewEmptyFile()

	// call the function under test with a non-existent local variable name
	err := updateLocal(ctx, file, "my_var", "my_value")

	// assert that the function returns an error
	assert.Error(t, err)

	// assert that the error message contains the expected substring
	assert.Contains(t, err.Error(), "local variable 'my_var' not found")
}

func TestSaveHclWithError(t *testing.T) {

	// Create a logger that writes to a buffer
	buf := bytes.Buffer{}
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	ctx := logger.WithContext(context.Background())

	// Create an invalid file handle (nil pointer) to cause an error
	var tempFile *os.File

	// Create a new HCL file
	hclFile := hclwrite.NewFile()

	// Write some data to the HCL file
	block := hclFile.Body().AppendNewBlock("resource", []string{"aws_s3_bucket", "my_bucket"})
	block.Body().SetAttributeValue("bucket", cty.StringVal("my-bucket-name"))

	// Call the function under test, passing the logger and temporary file handle
	err := saveHCLToFile(tempFile, ctx, hclFile)

	// Check that an error was returned
	assert.Error(t, err, "No error returned by function")
}

func TestParseHclFile(t *testing.T) {
	// create a temporary file with some HCL content
	tmpFile, err := os.CreateTemp("", "testfile-*.hcl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString(`resource "aws_s3_bucket" "my_bucket" {
  bucket = "my-bucket-name"
}`)
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	// open the temporary file for reading
	file, err := os.Open(tmpFile.Name())
	assert.NoError(t, err)
	defer file.Close()

	// Create a logger
	buf := bytes.Buffer{}
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	ctx := logger.WithContext(context.Background())

	// call the function under test
	hclFile, err := parseHclFile(ctx, file)

	// assert that the function returns no error
	assert.NoError(t, err)

	// assert that the HCL file contains the expected block
	assert.Equal(t, 1, len(hclFile.Body().Blocks()), "File contains more than expected configuration block")
	block := hclFile.Body().Blocks()[0]
	assert.Equal(t, "resource", block.Type())
	assert.Equal(t, []string{"aws_s3_bucket", "my_bucket"}, block.Labels())
	attr := block.Body().GetAttribute("bucket")
	assert.NotNil(t, attr)
}

func TestParseHclFileInvalidFormat(t *testing.T) {
	// create a temporary file with invalid HCL content
	tmpFile, err := os.CreateTemp("", "testfile-*.hcl")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString(`resource "aws_s3_bucket" "my_bucket" {
  bucket = "my-bucket-name"
`)
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	// open the temporary file for reading
	file, err := os.Open(tmpFile.Name())
	assert.NoError(t, err)
	defer file.Close()

	// call the function under test
	hclFile, err := parseHclFile(context.Background(), file)

	// assert that the function returns an error
	assert.Error(t, err)
	assert.Nil(t, hclFile)

	// assert that the error message contains the expected substring
	assert.Contains(t, err.Error(), "failed to parse file content")
}

func TestParseHclFileNilFile(t *testing.T) {

	// pass in a nil file
	var tmpFile *os.File

	// call the function under test
	hclFile, err := parseHclFile(context.Background(), tmpFile)

	// assert that the function returns an error
	assert.Error(t, err)
	assert.Nil(t, hclFile)

	// assert that the error message contains the expected substring
	assert.Contains(t, err.Error(), "failed to get file info")
}
