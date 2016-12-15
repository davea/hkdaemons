package main

import (
	"encoding/json"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/log"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
	"io/ioutil"
	"strconv"
)

func main() {
	log.Verbose = true
	log.Info = true

	file, err := ioutil.ReadFile("./hkthermostatd.json")
	if err != nil {
		panic(err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(file, &config); err != nil {
		panic(err)
	}

	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			log.Println(err)
		},
	})

	defer cli.Terminate()

	err = cli.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  config["broker"].(string),
		ClientID: []byte(config["client_id"].(string)),
	})
	if err != nil {
		panic(err)
	}

	var accessories []*accessory.Accessory

	configArr, ok := config["accessories"].([]interface{})
	if !ok {
		panic(err)
	}

	for _, accessory_config := range configArr {
		accessory_config := accessory_config.(map[string]interface{})
		info := accessory.Info{
			Name:         accessory_config["name"].(string),
			Manufacturer: config["manufacturer"].(string),
		}
		state_topic := fmt.Sprintf("sensor/temperature/%s", accessory_config["id"].(string))

		acc := accessory.NewTemperatureSensor(info, 0, -20, 40, 1.0)

		err = cli.Subscribe(&client.SubscribeOptions{
			SubReqs: []*client.SubReq{
				&client.SubReq{
					TopicFilter: []byte(state_topic),
					QoS:         mqtt.QoS0,
					// Define the processing of the message handler.
					Handler: func(topicName, message []byte) {
						new_value, err := strconv.ParseFloat(string(message), 64)
						if err != nil {
							panic(err)
						}
						log.Printf("%s value externally changed to %f\n", string(topicName), new_value)
						acc.TempSensor.CurrentTemperature.SetValue(new_value)
					},
				},
			},
		})
		if err != nil {
			panic(err)
		}

		accessories = append(accessories, acc.Accessory)
	}

	transport_config := hc.Config{Pin: config["pin"].(string), StoragePath: config["storage_path"].(string)}
	t, err := hc.NewIPTransport(transport_config, accessories[0], accessories[1:]...)
	if err != nil {
		log.Fatal(err)
	}

	hc.OnTermination(func() {
		t.Stop()
	})

	t.Start()
}
