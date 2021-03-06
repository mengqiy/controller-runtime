/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/appscode/jsonpatch"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook/types"
)

// Handler can handle an AdmissionRequest.
type Handler interface {
	Handle(context.Context, atypes.Request) atypes.Response
}

// HandlerFunc implements Handler interface using a single function.
type HandlerFunc func(context.Context, atypes.Request) atypes.Response

var _ Handler = HandlerFunc(nil)

// Handle process the AdmissionRequest by invoking the underlying function.
func (f HandlerFunc) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	return f(ctx, req)
}

// Webhook represents each individual webhook.
type Webhook struct {
	// Name is the name of the webhook
	Name string
	// Type is the webhook type, i.e. mutating, validating
	Type types.WebhookType
	// Path is the path this webhook will serve.
	Path string
	// Handlers contains a list of handlers. Each handler may only contains the business logic for its own feature.
	// For example, feature foo and bar can be in the same webhook if all the other configurations are the same.
	// The handler will be invoked sequentially as the order in the list.
	// Note: if you are using mutating webhook with multiple handlers, it's your responsibility to
	// ensure the handlers are not generating conflicting JSON patches.
	Handlers []Handler

	once sync.Once
}

func (w *Webhook) setDefaults() {
	if len(w.Name) == 0 {
		reg := regexp.MustCompile("[^a-zA-Z0-9]+")
		processedPath := strings.ToLower(reg.ReplaceAllString(w.Path, ""))
		w.Name = processedPath + ".example.com"
	}
}

// Add adds additional handler(s) in the webhook
func (w *Webhook) Add(handlers ...Handler) {
	w.Handlers = append(w.Handlers, handlers...)
}

// Webhook implements Handler interface.
var _ Handler = &Webhook{}

// Handle processes AdmissionRequest.
// If the webhook is mutating type, it delegates the AdmissionRequest to each handler and merge the patches.
// If the webhook is validating type, it delegates the AdmissionRequest to each handler and
// deny the request if anyone denies.
func (w *Webhook) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	if req.AdmissionRequest == nil {
		return ErrorResponse(http.StatusBadRequest, errors.New("got an empty AdmissionRequest"))
	}
	var resp atypes.Response
	switch w.Type {
	case types.WebhookTypeMutating:
		resp = w.handleMutating(ctx, req)
	case types.WebhookTypeValidating:
		resp = w.handleValidating(ctx, req)
	default:
		return ErrorResponse(http.StatusInternalServerError, errors.New("you must specify your webhook type"))
	}
	resp.Response.UID = req.AdmissionRequest.UID
	return resp
}

func (w *Webhook) handleMutating(ctx context.Context, req atypes.Request) atypes.Response {
	patches := []jsonpatch.JsonPatchOperation{}
	for _, handler := range w.Handlers {
		resp := handler.Handle(ctx, req)
		if !resp.Response.Allowed {
			setStatusOKInAdmissionResponse(resp.Response)
			return resp
		}
		if resp.Response.PatchType != nil && *resp.Response.PatchType != admissionv1beta1.PatchTypeJSONPatch {
			return ErrorResponse(http.StatusInternalServerError,
				fmt.Errorf("unexpected patch type returned by the handler: %v, only allow: %v",
					resp.Response.PatchType, admissionv1beta1.PatchTypeJSONPatch))
		}
		patches = append(patches, resp.Patches...)
	}
	var err error
	marshaledPatch, err := json.Marshal(patches)
	if err != nil {
		return ErrorResponse(http.StatusBadRequest, fmt.Errorf("error when marshaling the patch: %v", err))
	}
	return atypes.Response{
		Response: &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Code: http.StatusOK,
			},
			Patch:     marshaledPatch,
			PatchType: func() *admissionv1beta1.PatchType { pt := admissionv1beta1.PatchTypeJSONPatch; return &pt }(),
		},
	}
}

func (w *Webhook) handleValidating(ctx context.Context, req atypes.Request) atypes.Response {
	for _, handler := range w.Handlers {
		resp := handler.Handle(ctx, req)
		if !resp.Response.Allowed {
			setStatusOKInAdmissionResponse(resp.Response)
			return resp
		}
	}
	return atypes.Response{
		Response: &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Code: http.StatusOK,
			},
		},
	}
}

func setStatusOKInAdmissionResponse(resp *admissionv1beta1.AdmissionResponse) {
	if resp == nil {
		return
	}
	if resp.Result == nil {
		resp.Result = &metav1.Status{}
	}
	if resp.Result.Code == 0 {
		resp.Result.Code = http.StatusOK
	}
}

// GetName returns the name of the webhook.
func (w *Webhook) GetName() string {
	w.once.Do(w.setDefaults)
	return w.Name
}

// GetPath returns the path that the webhook registered.
func (w *Webhook) GetPath() string {
	w.once.Do(w.setDefaults)
	return w.Path
}

// GetType returns the type of the webhook.
func (w *Webhook) GetType() types.WebhookType {
	w.once.Do(w.setDefaults)
	return w.Type
}

// Handler returns a http.Handler for the webhook
func (w *Webhook) Handler() http.Handler {
	w.once.Do(w.setDefaults)
	return w
}

// Validate validates if the webhook is valid.
func (w *Webhook) Validate() error {
	w.once.Do(w.setDefaults)
	if len(w.Name) == 0 {
		return errors.New("field Name should not be empty")
	}
	if w.Type != types.WebhookTypeMutating && w.Type != types.WebhookTypeValidating {
		return fmt.Errorf("unsupported Type: %v, only WebhookTypeMutating and WebhookTypeValidating are supported", w.Type)
	}
	if len(w.Path) == 0 {
		return errors.New("field Path should not be empty")
	}
	if len(w.Handlers) == 0 {
		return errors.New("field Handler should not be empty")
	}
	return nil
}

var _ inject.Injector = &Webhook{}

// InjectFunc injects dependencies into the handlers.
func (w *Webhook) InjectFunc(f inject.Func) error {
	for _, handler := range w.Handlers {
		if err := f(handler); err != nil {
			return err
		}
	}
	return nil
}
