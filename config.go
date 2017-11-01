package eslogview

import (
	"io"
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ContextFields       []string `yaml:"context-fields"`
	TimestampField      string   `yaml:"timestamp-field"`
	ElasticSearchURL    string   `yaml:"elasticsearch-url"`
	MessageField        string   `yaml:"message-field"`
	Index               string   `yaml:"index"`
	MessageCleanupRegex string   `yaml:"message-cleanup-regex"`
	IgnoreBlankLogLines bool     `yaml:"ignore-blanks"`
}

func MustLoadConfig(conf io.Reader) Config {
	var config Config

	yamlConf, err := ioutil.ReadAll(conf)
	panicIf(err)

	err = yaml.Unmarshal(yamlConf, &config)
	panicIf(err)

	return config
}

func panicIf(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
