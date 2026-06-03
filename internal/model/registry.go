package model

import (
	"fmt"
	"sort"
)

const urlPattern = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/%s"

type Info struct {
	Size   int64
	SHA256 string
}

// knownModels holds authoritative size + sha256 from the Hugging Face LFS
// pointers (ggerganov/whisper.cpp), used to verify downloads strictly.
var knownModels = map[string]Info{
	"tiny":                {Size: 77691713, SHA256: "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21"},
	"base":                {Size: 147951465, SHA256: "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe"},
	"small":               {Size: 487601967, SHA256: "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b"},
	"medium":              {Size: 1533763059, SHA256: "6c14d5adee5f86394037b4e4e8b59f1673b6cee10e3cf0b11bbdbee79c156208"},
	"large-v3":            {Size: 3095033483, SHA256: "64d182b440b98d5203c4f9bd541544d84c605196c4f7b845dfa11fb23594d1e2"},
	"large-v3-turbo":      {Size: 1624555275, SHA256: "1fc70f774d38eb169993ac391eea357ef47c88757ef72ee5943879b7e8e2bc69"},
	"large-v3-turbo-q5_0": {Size: 574041195, SHA256: "394221709cd5ad1f40c46e6031ca61bce88931e6e088c188294c6d5a55ffa7e2"},
	"large-v3-turbo-q8_0": {Size: 874188075, SHA256: "317eb69c11673c9de1e1f0d459b253999804ec71ac4c23c17ecf5fbe24e259a1"},
}

func Filename(name string) string { return "ggml-" + name + ".bin" }

func DownloadURL(name string) string { return fmt.Sprintf(urlPattern, Filename(name)) }

// KnownNames returns the curated model names sorted for display.
func KnownNames() []string {
	names := make([]string, 0, len(knownModels))
	for n := range knownModels {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
