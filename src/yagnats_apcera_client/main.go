package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
)

type NatsInformation struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
}

type Config struct {
	PublishIntervalInSeconds float64           `yaml:"publish_interval_in_seconds"`
	Name                     string            `yaml:"name"`
	PayloadSizeInBytes       int               `yaml:"payload_size_in_bytes"`
	NatsServers              []NatsInformation `yaml:"nats_servers"`
	Population               int               `yaml:"population"`
	APIKey                   string            `yaml:"datadog_api_key"`
	Socket                   string            `yaml:"socket"`
}

func dedup(xs *[]string) {
	found := make(map[string]bool)
	j := 0
	for i, x := range *xs {
		if !found[x] {
			found[x] = true
			(*xs)[j] = (*xs)[i]
			j++
		}
	}
	*xs = (*xs)[:j]
}

var config Config

func main() {
	var configPath = flag.String("c", "/var/vcap/jobs/yagnats_apcera_client/config/yagnats_apcera_client.yml", "config path")
	flag.Parse()
	config = InitConfigFromFile(*configPath)

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
		communicateMetric([]byte(fmt.Sprintf("received---%s", string(msg.Data))))
		if matched, _ := regexp.Match("^publish--", msg.Data); matched {
			publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Data))
			client.Publish("yagnats.apcera.publish", publishMessage)
		}
	})

	count := 0

	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))
		//publishRequestMessage := []byte(fmt.Sprintf("request--%s--%d--", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		//publishRequestMessage = padMessage(publishRequestMessage, config.PayloadSizeInBytes)

		// logger.Info(fmt.Sprintf("publishing %s\n", publishMessage))
		// logger.Info(fmt.Sprintf("completed %d messages. outstanding: %d", msgsCompleted, len(messageTally)))
		client.Publish("yagnats.apcera.publish", publishMessage)
		communicateMetric([]byte(fmt.Sprintf("sent---%s", string(publishMessage))))

		//logger.Info(fmt.Sprintf("requesting %s\n", publishRequestMessage))
		//client.PublishRequest("yagnats.apcera.request", "yagnats.apcera.reply", publishRequestMessage)

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds * float64(time.Second)))
	}
}

func communicateMetric(message []byte) {
	c, err := net.Dial("unix", config.Socket)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	_, err = c.Write(message)
	if err != nil {
		panic(err)
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

func InitConfigFromFile(path string) Config {
	var e error
	c := Config{}

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
