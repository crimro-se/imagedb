package main

import (
	"fmt"
	"runtime"

	"gopkg.in/ini.v1"
)

type Config struct {
	API_SERVER             string
	IMAGE_SIZE_EMBEDDING   int
	IMAGE_SIZE_THUMBNAIL   int
	THREADS_FOR_THUMBNAILS int
	THREADS_FOR_INDEXING   int
	QUERY_RESULTS          int
}

func LoadConfig(path string) (*Config, error) {
	// default config
	config := &Config{
		API_SERVER:             "",
		IMAGE_SIZE_EMBEDDING:   336,
		IMAGE_SIZE_THUMBNAIL:   192,
		THREADS_FOR_THUMBNAILS: max(runtime.NumCPU()-4, 2),
		THREADS_FOR_INDEXING:   max(runtime.NumCPU()-4, 2),
		QUERY_RESULTS:          64,
	}
	cfgFile, err := ini.Load(path)
	if err != nil {
		return config, err
	}
	err = cfgFile.MapTo(config)
	fmt.Println("Loaded config:")
	fmt.Println(config)
	return config, err
}
