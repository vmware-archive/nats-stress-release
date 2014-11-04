package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
	"github.com/apcera/nats"
)

type NatsInformation struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
}

type Config struct {
	PublishIntervalInSeconds int               `yaml:"publish_interval_in_seconds"`
	Name                     string            `yaml:"name"`
	PayloadSizeInBytes       int               `yaml:"payload_size_in_bytes"`
	NatsServers              []NatsInformation `yaml:"nats_servers"`
}

func main() {
	var configPath = flag.String("c", "/var/vcap/jobs/yagnats_apcera_client/config/yagnats_apcera_client.yml", "config path")
	flag.Parse()
	config := InitConfigFromFile(*configPath)

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_apcera_client/yagnats_apcera_client.log"),
		},
		Level:     gosteno.LOG_INFO,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true,
	}
	gosteno.Init(c)
	logger := gosteno.NewLogger(config.Name)

	urls := make([]string, len(config.NatsServers))
	for _, server := range config.NatsServers {
		urls = append(urls, fmt.Sprintf("nats://%s:%s@%s:%d", server.Username, server.Password, server.Host, server.Port))
	}

	client, err := yagnats.Connect(urls)
	if err != nil {
		logger.Error(err.Error())
		panic(err.Error())
	}

	client.Subscribe(">", func(msg *nats.Msg) {
		logger.Info(fmt.Sprintf("Got message: %s\n", msg.Data))
	})

	count := 0
	for {
		publishMessage := []byte(fmt.Sprintf("publish_%s_%d_%s", config.Name, count, time.Now().UTC()))
		publishRequestMessage := []byte(fmt.Sprintf("request_%s_%d_%s", config.Name, count, time.Now().UTC()))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		publishRequestMessage = padMessage(publishRequestMessage, config.PayloadSizeInBytes)

		client.Publish("yagnats.apcera.publish", publishMessage)
		logger.Info(fmt.Sprintf("Published message: %s\n", publishMessage))

		client.PublishRequest("yagnats.apcera.request", "yagnats.apcera.reply", publishRequestMessage)
		logger.Info(fmt.Sprintf("Requested message: %s\n", publishRequestMessage))

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds) * time.Second)
	}
}

func padMessage(message []byte, paddingLength int) []byte {
	if len(message) < paddingLength {
		a := make([]byte, paddingLength)
		for i := range a {
			a[i] = []byte(".")[0]
		}
		copy(a, message)
		return a
	}
	return message
}

func InitConfigFromFile(path string) *Config {
	var c *Config = new(Config)
	var e error

	b, e := ioutil.ReadFile(path)
	if e != nil {
		panic(e.Error())
	}

	e = c.Initialize(b)
	if e != nil {
		panic(e.Error())
	}

	return c
}

func (c *Config) Initialize(configYAML []byte) error {
	return candiedyaml.Unmarshal(configYAML, &c)
}
