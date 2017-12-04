package lats_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/instrumented_handler"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sending HTTP events through loggregator", func() {
	Context("When the instrumented handler receives a request", func() {
		It("should emit an HttpStartStop through the firehose", func() {
			msgChan, errorChan := ConnectToFirehose()

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("localhost:%d", config.DropsondePort))
			Expect(err).ToNot(HaveOccurred())
			origin := fmt.Sprintf("%s-%d", OriginName, time.Now().UnixNano())
			emitter := emitter.NewEventEmitter(udpEmitter, origin)
			done := make(chan struct{})
			handler := instrumented_handler.InstrumentedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				close(done)
			}), emitter)

			r, err := http.NewRequest("HEAD", "/", nil)
			Expect(err).ToNot(HaveOccurred())
			r.Header.Add("User-Agent", "Spider-Man")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			Eventually(done).Should(BeClosed())

			receivedEnvelope := FindMatchingEnvelopeByOrigin(msgChan, origin)
			Expect(receivedEnvelope).NotTo(BeNil())
			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_HttpStartStop))

			event := receivedEnvelope.GetHttpStartStop()
			Expect(event.GetPeerType().String()).To(Equal(events.PeerType_Server.Enum().String()))
			Expect(event.GetMethod().String()).To(Equal(events.Method_HEAD.Enum().String()))
			Expect(event.GetStartTimestamp()).ToNot(BeZero())
			Expect(event.GetStopTimestamp()).ToNot(BeZero())
			Expect(event.GetUserAgent()).To(Equal("Spider-Man"))
			Expect(event.GetStatusCode()).To(BeEquivalentTo(http.StatusTeapot))

			Expect(errorChan).To(BeEmpty())
		})

		It("should emit httpStartStop events for specific apps to the stream endpoint", func() {
			id := "e0f50f22-d93a-11e7-9296-cec278b6b50a"
			msgChan, errorChan := ConnectToStream(id)

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("localhost:%d", config.DropsondePort))
			Expect(err).ToNot(HaveOccurred())
			emitter := emitter.NewEventEmitter(udpEmitter, OriginName)
			r, err := http.NewRequest("HEAD", "/", nil)
			Expect(err).ToNot(HaveOccurred())
			r.Header.Add("User-Agent", "Superman")
			r.Header.Add("X-CF-ApplicationID", id)
			done := make(chan struct{})
			handler := instrumented_handler.InstrumentedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				close(done)
			}), emitter)

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			Eventually(done).Should(BeClosed())

			receivedEnvelope, err := FindMatchingEnvelopeByID(id, msgChan)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedEnvelope).NotTo(BeNil())
			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_HttpStartStop))

			event := receivedEnvelope.GetHttpStartStop()
			Expect(GetAppId(receivedEnvelope)).To(Equal(id))
			Expect(event.GetPeerType().String()).To(Equal(events.PeerType_Server.Enum().String()))
			Expect(event.GetMethod().String()).To(Equal(events.Method_HEAD.Enum().String()))
			Expect(event.GetStartTimestamp()).ToNot(BeZero())
			Expect(event.GetStopTimestamp()).ToNot(BeZero())
			Expect(event.GetUserAgent()).To(Equal("Superman"))
			Expect(event.GetStatusCode()).To(BeEquivalentTo(http.StatusTeapot))

			Expect(errorChan).To(BeEmpty())
		})
	})
})
