package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

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
	var configPath = flag.String("c", "/var/vcap/jobs/og_client/config/og_client.yml", "config path")
	flag.Parse()
	config = InitConfigFromFile(*configPath)

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_og_client/yagnats_og_client.log"),
		},
		Level:     gosteno.LOG_INFO,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true,
	}
	gosteno.Init(c)
	logger := gosteno.NewLogger(config.Name)

	connectionCluster := &yagnats.ConnectionCluster{}
	for _, server := range config.NatsServers {
		connectionCluster.Members = append(connectionCluster.Members, &yagnats.ConnectionInfo{
			Addr:     fmt.Sprintf("%s:%d", server.Host, server.Port),
			Username: server.Username,
			Password: server.Password,
		})
	}

	client := yagnats.NewClient()
	err := client.Connect(connectionCluster)
	if err != nil {
		logger.Error(err.Error())
		panic(err.Error())
	}

	client.Subscribe(">", func(msg *yagnats.Message) {
		err := communicateMetric([]byte(fmt.Sprintf("received---%s", string(msg.Payload))))
		if err != nil {
			logger.Error(err.Error())
		}
		if matched, _ := regexp.Match("^publish--", msg.Payload); matched {
			publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Payload))
			client.Publish("yagnats.og.publish", publishMessage)
		}
	})

	count := 0

	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))
		// publishWithReplyMessage := []byte(fmt.Sprintf("request--%s--%d--", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		// publishWithReplyMessage = padMessage(publishWithReplyMessage, config.PayloadSizeInBytes)

		// logger.Info(fmt.Sprintf("publishing %s\n", publishMessage))
		// logger.Info(fmt.Sprintf("completed %d messages. outstanding: %d", msgsCompleted, len(messageTally)))
		client.Publish("yagnats.og.publish", publishMessage)
		err := communicateMetric([]byte(fmt.Sprintf("sent---%s", string(publishMessage))))
		if err != nil {
			logger.Error(err.Error())
		}

		// logger.Info(fmt.Sprintf("requesting %s\n", publishWithReplyMessage))
		// client.PublishWithReplyTo("yagnats.og.request", "yagnats.og.reply", publishWithReplyMessage)

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds * float64(time.Second)))
	}
}

// func communicateMetric(message []byte) {
// 	c, err := net.DialTimeout("unix", config.Socket, 10*time.Second)
// 	processNetErr(err)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer c.Close()
// 	_, err = c.Write(message)
// 	if err != nil {
// 		panic(err)
// 	}
// }
func communicateMetric(message []byte) error {
	//c, err := net.DialTimeout("unix", config.Socket, 10*time.Second)
	resp, err := http.Post("http://127.0.0.1:4568/messages", "application/text", bytes.NewReader(message))
	resp.Body.Close()
	return err
	//defer c.Close()
	//_, err = c.Write(message)
	// if err != nil {
	// 	panic(err)
	// }
}

// func processNetErr(err error) {
// 	if err != nil {

// 		// print error string e.g.
// 		// "read tcp example.com:80: resource temporarily unavailable"
// 		fmt.Printf("reader %v\n", err)

// 		// print type of the error, e.g. "*net.OpError"
// 		fmt.Printf("%T", err)

// 		//if err == os.EINVAL {
// 		// socket is not valid or already closed
// 		//fmt.Println("EINVAL")
// 		//}
// 		// if err == os.EOF {
// 		// 	// remote peer closed socket
// 		// 	fmt.Println("EOF")
// 		// }

// 		// matching rest of the codes needs typecasting, errno is
// 		// wrapped on OpError
// 		if e, ok := err.(*net.OpError); ok {
// 			// print wrapped error string e.g.
// 			// "os.Errno : resource temporarily unavailable"
// 			fmt.Printf("%T : %v\n", e.Error, e.Error)
// 			if e.Timeout() {
// 				// is this timeout error?
// 				fmt.Println("TIMEOUT")
// 			}
// 			if e.Temporary() {
// 				// is this temporary error?  True on timeout,
// 				// socket interrupts or when buffer is full
// 				fmt.Println("TEMPORARY")
// 			}

// 			// specific granular error codes in case we're interested
// 			// switch e.Error {
// 			// case os.EAGAIN:
// 			// 	// timeout
// 			// 	fmt.Println("EAGAIN")
// 			// case os.EPIPE:
// 			// 	// broken pipe (e.g. on connection reset)
// 			// 	fmt.Println("EPIPE")
// 			// default:
// 			// 	// just write raw errno code, can be platform specific
// 			// 	// (see syscall for definitions)
// 			// 	fmt.Printf("%d\n", int64(e.Error.(os.Errno)))
// 			// }
// 		}
// 	}
// }

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
