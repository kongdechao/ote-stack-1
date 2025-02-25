/*
Copyright 2019 Baidu, Inc.

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

// Package edgehandler maintenances the websocket connection with cloud server
// and process the receive messages.
package edgehandler

import (
	"fmt"
	"net/http"
	"time"

	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clusterselector"
	"github.com/baidu/ote-stack/pkg/config"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

// EdgeHandler is edgehandler interface that process messages from tunnel and transmit to shim.
type EdgeHandler interface {
	// Start will start edgehandler.
	Start() error
}

// edgeHandler processes message from tunnel and transmit to shim.
type edgeHandler struct {
	conf       *config.ClusterControllerConfig
	edgeTunnel tunnel.EdgeTunnel
	shimClient shimServiceClient
}

// NewEdgeHandler returns a edgeHandler object.
func NewEdgeHandler(c *config.ClusterControllerConfig) EdgeHandler {
	return &edgeHandler{conf: c}
}

func (e *edgeHandler) valid() error {
	if e.conf.ClusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}
	if e.conf.K8sClient == nil && !e.isRemoteShim() {
		return fmt.Errorf("k8s client is unavailable or remoteshim not set")
	}
	if e.conf.ParentCluster == "" {
		return fmt.Errorf("parent cluster is empty")
	}
	return nil
}

func (e *edgeHandler) isRoot() bool {
	return config.IsRoot(e.conf.ClusterName)
}

func (e *edgeHandler) isRemoteShim() bool {
	return e.conf.RemoteShimAddr != ""
}

func (e *edgeHandler) Start() error {
	if e.isRoot() {
		klog.Infof("will not start edgehandler for root cluster")
		return nil
	}

	if err := e.valid(); err != nil {
		return err
	}

	if e.isRemoteShim() {
		klog.Infof("init remote shim client")
		e.shimClient = newRemoteShimClient(e.conf.RemoteShimAddr)
	} else {
		klog.Infof("init local shim client")
		e.shimClient = newLocalShimClient(e.conf)
	}

	if e.shimClient == nil {
		return fmt.Errorf("fail to init shim client")
	}

	e.edgeTunnel = tunnel.NewEdgeTunnel(e.conf)
	e.edgeTunnel.RegistReceiveMessageHandler(e.receiveMessageFromTunnel)
	if err := e.edgeTunnel.Start(); err != nil {
		return err
	}

	go e.sendMessageToTunnel()
	return nil
}

func (e *edgeHandler) sendMessageToTunnel() {
	for {
		cc := <-e.conf.ClusterToEdgeChan
		data, err := cc.Serialize()
		if err != nil {
			continue
		}
		go e.edgeTunnel.Send(data)
	}
}

func (e *edgeHandler) receiveMessageFromTunnel(client string, message []byte) (ret error) {
	ret = nil
	data, err := otev1.ClusterControllerDeserialize(message)
	if err != nil {
		ret = fmt.Errorf("can not deserialize message, error: %s", err.Error())
		klog.Error(ret)
		return
	}

	e.conf.EdgeToClusterChan <- *data

	selector := clusterselector.NewSelector(data.Spec.ClusterSelector)
	if selector.Has(e.conf.ClusterName) {
		e.handleMessage(data)
	}

	return
}

func responseErrorStatus(err error) *otev1.ClusterControllerStatus {
	return &otev1.ClusterControllerStatus{
		Timestamp:  time.Now().Unix(),
		Body:       err.Error(),
		StatusCode: http.StatusInternalServerError,
	}
}

func (e *edgeHandler) handleMessage(c *otev1.ClusterController) error {
	var (
		status *otev1.ClusterControllerStatus
	)

	if c.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_REGIST_CLUSTER ||
		c.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_UNREGIST_CLUSTER ||
		c.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_CLUSTER_ROUTE {
		return nil
	}

	// dispatch to target shim.
	klog.V(1).Infof("dispatch message %v to %s", c, c.Spec.Destination)
	req := clusterControllerSpec2Pb(&c.Spec)
	resp, err := e.shimClient.Do(req)

	if err != nil {
		status = responseErrorStatus(err)
		klog.Errorf("handleTask error: %s", err.Error())
	} else {
		status = pb2ClusterControllerStatus(resp)
	}

	// package response message.
	c.Status = make(map[string]otev1.ClusterControllerStatus)
	c.Status[e.conf.ClusterName] = *status

	// send to cloudtunnel.
	data, err := c.Serialize()
	if err != nil {
		klog.Errorf("marshal ClusterController error: %s", err.Error())
		return err
	}
	go e.edgeTunnel.Send(data)

	return nil
}
