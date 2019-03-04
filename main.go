package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	Registry     = prometheus.NewRegistry()
	InputCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "input_current_ampere",
			Help: "Current current provided by PV to the inverter, partitioned by array.",
		},
		[]string{"array"},
	)
	InputVoltage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "input_voltage",
			Help: "Current voltage provided by PV to the inverter, partitioned by array.",
		},
		[]string{"array"},
	)
	OutputCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "output_current_ampere",
			Help: "Current current provided by the inverter to the grid.",
		},
	)
	OutputVoltage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "output_voltage",
			Help: "Current voltage provided by the inverter to the grid.",
		},
	)
	OutputPower = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "output_power_watt",
			Help: "Current power provided by the inverter to the grid.",
		},
	)
	TotalEnergyValue float64
	TotalEnergy      = prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name: "total_energy_kwh",
			Help: "The total energy provided by this inverter.",
		},
		func() float64 {
			return TotalEnergyValue
		},
	)
	Frequency = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "grid_frequency_hertz",
			Help: "Current frequency detected in the grid.",
		},
	)
	Measuring = true
)

func init() {
	Registry.MustRegister(InputCurrent, InputVoltage, OutputCurrent, OutputVoltage, OutputPower, TotalEnergy, Frequency)
}

// {"type":"X1-Boost-Air-Mini","SN":"SWNBHLDR9Q","ver":"2.32.6","Data":[0.6,0.6,203.8,200.7,1.5,236.3,238,33,3.2,6.7,0,142,129,0.00,0.00,0,0,0,0.0,0.0,0.00,0.00,0,0,0,0.0,0.0,0.00,0.00,0,0,0,0.0,0.0,0,0,0,0,0,0,0,0.00,0.00,0,0,0,0,0,0,0,49.98,0,0,0,0,0,0,0,0,0,0.00,0,8,0,0,0.00,0,8,2],"Information":[3.680,4,"X1-Boost-Air-Mini","XB362188276114",1,3.25,1.09,1.12,0.00]}

type jsonResult struct {
	Type    string    `json:"type"`
	Serial  string    `json:"SN"`
	Version string    `json:"ver"`
	Data    []float64 `json:"Data"`
}

func setMeasurements(data []float64) {
	if len(data) < 51 {
		return
	}
	InputCurrent.WithLabelValues("1").Set(data[0])
	InputCurrent.WithLabelValues("2").Set(data[1])
	InputVoltage.WithLabelValues("1").Set(data[2])
	InputVoltage.WithLabelValues("2").Set(data[3])
	OutputCurrent.Set(data[4])
	OutputPower.Set(data[6])
	if data[50] > 0 {
		if !Measuring {
			Registry.MustRegister(OutputVoltage, TotalEnergy, Frequency)
			Measuring = true
		}
		OutputVoltage.Set(data[5])
		TotalEnergyValue = data[9]
		Frequency.Set(data[50])
	} else {
		if Measuring {
			Registry.Unregister(OutputVoltage)
			Registry.Unregister(TotalEnergy)
			Registry.Unregister(Frequency)
			Measuring = false
		}
	}
}

func main() {
	started := false
	http.Handle("/metrics", promhttp.HandlerFor(Registry, promhttp.HandlerOpts{}))
	server := &http.Server{}
	promListener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 9550})
	if err != nil {
		panic(err)
	}
	dataListener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 2901})
	if err != nil {
		panic(err)
	}
	conn, err := dataListener.Accept()
	buffer := bufio.NewReader(conn)
	for {
		for {
			b, err := buffer.ReadByte()
			if err != nil {
				panic(err)
			}
			if b == '{' {
				err = buffer.UnreadByte()
				if err != nil {
					panic(err)
				}
				break
			}
		}
		jsonData, err := buffer.ReadBytes('}')
		if err != nil {
			panic(err)
		}
		var data jsonResult
		if err := json.Unmarshal(jsonData, &data); err != nil {
			fmt.Println("Invalid packet.")
			continue
		}
		setMeasurements(data.Data)
		fmt.Printf("Total: %v\n", TotalEnergyValue)
		if !started {
			fmt.Println("Starting server")
			go server.Serve(promListener)
			started = true
		}
	}
}
