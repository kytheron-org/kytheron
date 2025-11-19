package config

import (
	"github.com/spf13/viper"
)

type Plugin struct {
	Type    string `yaml:"type"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Config struct {
	Plugins  map[string]Plugin `yaml:"plugins"`
	Policies Policies          `yaml:"policies"`
	Server   Server            `yaml:"server"`
	Registry Registry          `yaml:"registry"`
	Database Database          `yaml:"database"`
	Kafka    KafkaMap          `yaml:"kafka"`
	LogLevel string            `yaml:"logLevel"`
}

type Kafka struct {
	Url string `yaml:"url"`
}

type KafkaMap struct {
	Source Kafka `yaml:"source"`
	Parser Kafka `yaml:"parser"`
}

type Database struct {
	Url string `yaml:"url"`
}

type Server struct {
	Http HttpServer `yaml:"http"`
	Grpc GrpcServer `yaml:"grpc"`
}

type HttpServer struct {
	Port int `yaml:"port"`
}

type GrpcServer struct {
	Port               int `yaml:"port"`
	MaxSendMessageSize int `yaml:"maxSendMessageSize"`
	MaxRecvMessageSize int `yaml:"maxRecvMessageSize"`
}

type Registry struct {
	CacheDir string `yaml:"cache"`
}
type Policies struct {
	Url string `yaml:"url"`
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)

	// Bind environment variables
	viper.SetEnvPrefix("kytheron")

	// Find and read the config file
	err := viper.ReadInConfig()

	// Handle errors
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
