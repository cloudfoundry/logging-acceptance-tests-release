package lats_test

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	v2 "code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	Describe("emit v1 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			appID := randAppID()
			env := createLogEnvelopeV1("Recent log message", appID)
			EmitToMetronV1(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs(appID, "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}

			Eventually(getRecentLogs).Should(ContainElement(env.LogMessage))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			appID := randAppID()
			msgChan, errorChan := ConnectToStream(appID)

			env := createLogEnvelopeV1("Stream message", appID)
			EmitToMetronV1(env)

			receivedEnvelope, err := FindMatchingEnvelopeByID(appID, msgChan)
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedEnvelope.LogMessage).To(Equal(env.LogMessage))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v2 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			appID := randAppID()
			env := createLogEnvelopeV2("Recent log message", appID)
			EmitToMetronV2(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs(appID, "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}

			v1EnvLogMsg := &events.LogMessage{
				Message:        env.GetLog().Payload,
				MessageType:    events.LogMessage_OUT.Enum(),
				Timestamp:      proto.Int64(env.Timestamp),
				AppId:          proto.String(env.SourceId),
				SourceType:     proto.String(""),
				SourceInstance: proto.String(""),
			}

			Eventually(getRecentLogs).Should(ContainElement(v1EnvLogMsg))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			appID := randAppID()
			msgChan, errorChan := ConnectToStream(appID)

			env := createLogEnvelopeV2("Stream message", appID)
			EmitToMetronV2(env)

			receivedEnvelope, err := FindMatchingEnvelopeByID(appID, msgChan)
			Expect(err).NotTo(HaveOccurred())

			v1EnvLogMsg := &events.LogMessage{
				Message:        env.GetLog().Payload,
				MessageType:    events.LogMessage_OUT.Enum(),
				Timestamp:      proto.Int64(env.Timestamp),
				AppId:          proto.String(env.SourceId),
				SourceType:     proto.String(""),
				SourceInstance: proto.String(""),
			}

			Expect(receivedEnvelope.LogMessage).To(Equal(v1EnvLogMsg))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v1 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			appID := randAppID()
			msgChan := ReadFromRLP(appID, false)

			env := createLogEnvelopeV1("Stream message", appID)
			EmitToMetronV1(env)

			v2EnvLog := &v2.Log{
				Payload: env.GetLogMessage().Message,
				Type:    v2.Log_OUT,
			}

			giveUp := time.NewTimer(5 * time.Second)
			for {
				select {
				case e := <-msgChan:
					if reflect.DeepEqual(e.GetLog(), v2EnvLog) {
						// Success
						return
					}
				case <-giveUp.C:
					Fail("expected to receive LogMessage")
				}
			}
		})

		It("sends log messages through rlp with preferred tags", func() {
			appID := randAppID()
			msgChan := ReadFromRLP(appID, true)

			env := createLogEnvelopeV1("Stream message", appID)
			EmitToMetronV1(env)

			v2EnvLog := &v2.Log{
				Payload: env.GetLogMessage().Message,
				Type:    v2.Log_OUT,
			}

			giveUp := time.NewTimer(5 * time.Second)
			for {
				select {
				case e := <-msgChan:
					if reflect.DeepEqual(e.GetLog(), v2EnvLog) {
						// Success
						return
					}
				case <-giveUp.C:
					Fail("expected to receive LogMessage")
				}
			}
		})
	})

	Describe("emit v2 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			appID := randAppID()
			msgChan := ReadFromRLP(appID, false)

			env := createLogEnvelopeV2("Stream message", appID)
			EmitToMetronV2(env)

			giveUp := time.NewTimer(5 * time.Second)
			for {
				select {
				case e := <-msgChan:
					if reflect.DeepEqual(e.GetLog(), env.GetLog()) {
						// Success
						return
					}
				case <-giveUp.C:
					Fail("expected to receive LogMessage")
				}
			}
		})
	})
})

func createLogEnvelopeV1(message, appID string) *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_LogMessage.Enum(),
		Origin:    proto.String(OriginName),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte(message),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appID),
		},
	}
}

func createLogEnvelopeV2(message, appID string) *v2.Envelope {
	return &v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		DeprecatedTags: map[string]*v2.Value{
			"origin": {
				Data: &v2.Value_Text{
					Text: OriginName,
				},
			},
		},
		Message: &v2.Envelope_Log{
			Log: &v2.Log{
				Payload: []byte(message),
				Type:    v2.Log_OUT,
			},
		},
	}
}

func randAppID() string {
	return fmt.Sprintf("lats - %d", rand.Int63())
}
