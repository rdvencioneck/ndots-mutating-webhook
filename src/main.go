package main

import (
  "crypto/tls"
  "fmt"
  "net/http"
  "log"
  "os"
  "encoding/json"
  "io/ioutil"

  "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  corev1 "k8s.io/api/core/v1"
  admission "k8s.io/api/admission/v1"
)

var logger = log.New(os.Stdout, "", log.LstdFlags)

func validateAdmissionRequest(request *http.Request, deserializer runtime.Decoder) (*admission.AdmissionReview, error) {
	if request.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("[Error] Invalid Admission Request: content-type must be application/json")
	}

	var body []byte
	if request.Body != nil {
		requestData, err := ioutil.ReadAll(request.Body)
		if err != nil {
			return nil, fmt.Errorf("[Error] Invalid body: %s", err)
		}
		body = requestData
	}

	admissionReviewRequest := &admission.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, fmt.Errorf("[Error] unable to deserialize body: %s", err)
	}

	return admissionReviewRequest, nil
}

func handlePodMutation(response http.ResponseWriter, request *http.Request) {

	deserializer := serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	admissionReviewRequest, err := validateAdmissionRequest(request, deserializer)
	if err != nil {
    msg := fmt.Sprintf("[Error] Admission Review issue: %v", err)
		logger.Printf(msg)
		response.WriteHeader(400)
		response.Write([]byte(msg))
		return
	}

	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("[Error] Wrong resource, got: %s instead of a Pod", admissionReviewRequest.Request.Resource.Resource)
		logger.Printf(msg)
		response.WriteHeader(400)
		response.Write([]byte(msg))
		return
	}

	rawAdmissionReviewRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawAdmissionReviewRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("[Error] Couldn't decode raw pod: %v", err)
		logger.Printf(msg)
		response.WriteHeader(500)
		response.Write([]byte(msg))
		return
	}

	foundNdots := false
	if pod.Spec.DNSConfig != nil && len(pod.Spec.DNSConfig.Options) > 0 {
		for _, option := range pod.Spec.DNSConfig.Options {
			if option.Name == "ndots" {
				foundNdots = true
        logger.Printf(fmt.Sprintf("[%s/%s]: No changes needed, as it already includes ndots config.", pod.Namespace, pod.Name))
				break
			}
		}
	}

  var patch string

	ndots := os.Getenv("NDOTS")

	if !foundNdots {
		if pod.Spec.DNSConfig == nil {
			logger.Printf(fmt.Sprintf("[%s/%s]: Creating dnsConfig with ndots included.", pod.Namespace, pod.Name))
			patch = fmt.Sprintf(`[{"op": "add", "path": "/spec/dnsConfig", "value": {"options": [{"name": "ndots", "value": "%s"}]}}]`, ndots)
		} else if len(pod.Spec.DNSConfig.Options) == 0 {
			logger.Printf(fmt.Sprintf("[%s/%s]: dnsConfig found but no options. Creating ndots one.", pod.Namespace, pod.Name))
			patch = fmt.Sprintf(`[{"op": "add", "path": "/spec/dnsConfig/options", "value": [{"name": "ndots", "value": "%s"}]}]`, ndots)
		} else {
			logger.Printf(fmt.Sprintf("[%s/%s]: Including ndots within the existent options.", pod.Namespace, pod.Name))
			patch = fmt.Sprintf(`[{"op":"add","path":"/spec/dnsConfig/options/-","value":{"name":"ndots","value":"%s"}}]`, ndots)
		}
	}
  admissionResponse := &admission.AdmissionResponse{}
	admissionResponse.Allowed = true
	if patch != "" {
    patchType := admission.PatchTypeJSONPatch
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
	}

	var admissionReviewResponse admission.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		msg := fmt.Sprintf("[Error] Unable to marshal response json: %v", err)
		logger.Printf(msg)
		response.WriteHeader(500)
		response.Write([]byte(msg))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.Write(resp)
}

func main() {
  certificateFile := "/etc/certs/tls.crt"
  keyFile := "/etc/certs/tls.key"

  cert, err := tls.LoadX509KeyPair(certificateFile, keyFile)
  if err != nil {
    panic(err)
  }

  logger.Printf("Starting ndots-injector mutating webhook")

  http.HandleFunc("/mutate", handlePodMutation)
	server := http.Server{
		Addr: fmt.Sprintf(":%d", 443),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		ErrorLog: logger,
	}

	if err := server.ListenAndServeTLS("", ""); err != nil {
		panic(err)
	}
}