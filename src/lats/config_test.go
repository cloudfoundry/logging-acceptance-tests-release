package lats_test

import (
	"encoding/json"
	"errors"
	"log"
	"os"
)

type TestConfig struct {
	IP              string
	DopplerEndpoint string
	SkipSSLVerify   bool

	DropsondePort int

	MetronTLSClientConfig TLSClientConfig

	ReverseLogProxyAddr string
}

type TLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type MetronConfig struct {
	IncomingUDPPort          int
	LoggregatorDropsondePort int
	Index                    string
	Zone                     string
}

func Load() (*TestConfig, error) {
	path := os.Getenv("CONFIG")
	if path == "" {
		return nil, errors.New("Must set $CONFIG to point to an integration config .json file.")
	}

	configFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	config := &TestConfig{
		MetronTLSClientConfig: TLSClientConfig{
			CAFile:   "/var/vcap/jobs/metron_agent/config/certs/loggregator_ca.crt",
			CertFile: "/var/vcap/jobs/metron_agent/config/certs/metron_agent.crt",
			KeyFile:  "/var/vcap/jobs/metron_agent/config/certs/metron_agent.key",
		},
		ReverseLogProxyAddr: "reverse-log-proxy.service.cf.internal:8082",
	}
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	if config.DropsondePort == 0 {
		config.DropsondePort = 3457
	}

	if config.IP == "" {
		log.Panic("Config requires IP but is missing")
	}

	return config, nil
}

func (tc *TestConfig) SaveMetronConfig() {
	// TODO: Consider removing these default values and forcing user to
	// provide all values. These were initially added as a fixture file to get
	// bosh-lite lats passing. When we converted over to binary blobs the
	// fixture file had to go.
	metronConfig := MetronConfig{
		IncomingUDPPort:          3457,
		LoggregatorDropsondePort: 3457,
		Index: "0",
		Zone:  "z1",
	}

	metronConfig.IncomingUDPPort = tc.DropsondePort

	metronConfigFile, err := os.Create("fixtures/metron.json")
	bytes, err := json.Marshal(metronConfig)
	if err != nil {
		panic(err)
	}

	metronConfigFile.Write(bytes)
	metronConfigFile.Close()
}
