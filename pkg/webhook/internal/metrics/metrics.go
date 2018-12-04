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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// TotalRequests is a prometheus metric which counts the total number of requests that
	// the webhook server has received.
	TotalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_runtime_webhook_total_requests",
			Help: "Number of total webhook requests",
		},
		[]string{"webhook"},
	)
	// SucceededRequests is a prometheus metric which counts the total number of requests that
	// the webhook server has successfully processed.
	SucceededRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_runtime_webhook_succeeded_requests",
			Help: "Number of total succeeded webhook requests",
		},
		[]string{"webhook"},
	)
	// FailedRequests is a prometheus metric which counts the total number of requests that
	// the webhook server has failed to process.
	FailedRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_runtime_webhook_failed_requests",
			Help: "Number of total failed webhook requests",
		},
		[]string{"webhook"},
	)
	// InternalErrorRequests is a prometheus metric which counts the total number of requests that
	// the webhook server has failed to process with an server internal error.
	InternalErrorRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_runtime_webhook_internal_error_requests",
			Help: "Number of total failed webhook requests with server internal error",
		},
		[]string{"webhook"},
	)
	// Duration is a prometheus metric which keeps track of the Duration
	// of process a admission request.
	Duration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "controller_runtime_webhook_request_process_seconds",
			Help: "Length of time per request per webhook",
		},
		[]string{"webhook"},
	)
)

func init() {
	prometheus.MustRegister(
		TotalRequests,
		SucceededRequests,
		FailedRequests,
		Duration)
}
