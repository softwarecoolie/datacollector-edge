package http

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"github.com/streamsets/datacollector-edge/api"
	"github.com/streamsets/datacollector-edge/container/common"
	"github.com/streamsets/datacollector-edge/stages/stagelibrary"
	"io/ioutil"
	"log"
	"net/http"
)

const (
	DEBUG      = false
	LIBRARY    = "streamsets-datacollector-basic-lib"
	STAGE_NAME = "com_streamsets_pipeline_stage_destination_http_HttpClientDTarget"
)

type HttpClientDestination struct {
	*common.BaseStage
	resourceUrl           string
	headers               []interface{}
	singleRequestPerBatch bool
	httpCompression       string
	tlsEnabled            bool
	trustStoreFilePath    string
}

func init() {
	stagelibrary.SetCreator(LIBRARY, STAGE_NAME, func() api.Stage {
		return &HttpClientDestination{BaseStage: &common.BaseStage{}}
	})
}

func (h *HttpClientDestination) Init(stageContext api.StageContext) error {
	if err := h.BaseStage.Init(stageContext); err != nil {
		return err
	}
	stageConfig := h.GetStageConfig()
	log.Println("[DEBUG] HttpClientDestination Init method")
	for _, config := range stageConfig.Configuration {
		if config.Name == "conf.resourceUrl" {
			h.resourceUrl = stageContext.GetResolvedValue(config.Value).(string)
		}

		if config.Name == "conf.headers" {
			h.headers = stageContext.GetResolvedValue(config.Value).([]interface{})
		}

		if config.Name == "conf.singleRequestPerBatch" {
			h.singleRequestPerBatch = stageContext.GetResolvedValue(config.Value).(bool)
		}

		if config.Name == "conf.client.httpCompression" {
			h.httpCompression = stageContext.GetResolvedValue(config.Value).(string)
		}

		if config.Name == "conf.client.tlsConfig.tlsEnabled" {
			h.tlsEnabled = stageContext.GetResolvedValue(config.Value).(bool)
		}

		if config.Name == "conf.client.tlsConfig.trustStoreFilePath" {
			h.trustStoreFilePath = stageContext.GetResolvedValue(config.Value).(string)
		}
	}
	return nil
}

func (h *HttpClientDestination) Write(batch api.Batch) error {
	log.Println("[DEBUG] HttpClientDestination write method")
	var err error
	var batchByteArray []byte
	for _, record := range batch.GetRecords() {
		var recordByteArray []byte
		value := record.GetValue()
		switch value.(type) {
		case string:
			recordByteArray = []byte(value.(string))
		default:
			recordByteArray, err = json.Marshal(value)
			if err != nil {
				return err
			}
		}

		if h.singleRequestPerBatch {
			batchByteArray = append(batchByteArray, recordByteArray...)
			batchByteArray = append(batchByteArray, "\n"...)
		} else {
			err = h.sendToSDC(recordByteArray)
			if err != nil {
				return err
			}
		}
	}
	if h.singleRequestPerBatch && len(batch.GetRecords()) > 0 {
		err = h.sendToSDC(batchByteArray)
	}
	return err
}

func (h *HttpClientDestination) sendToSDC(jsonValue []byte) error {
	var buf bytes.Buffer

	if h.httpCompression == "GZIP" {
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(jsonValue); err != nil {
			return err
		}
		gz.Close()
	} else {
		buf = *bytes.NewBuffer(jsonValue)
	}

	req, err := http.NewRequest("POST", h.resourceUrl, &buf)
	if h.headers != nil {
		for _, header := range h.headers {
			req.Header.Set(header.(map[string]interface{})["key"].(string),
				header.(map[string]interface{})["value"].(string))
		}
	}

	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	if h.httpCompression == "GZIP" {
		req.Header.Set("Content-Encoding", "gzip")
	}

	var client *http.Client

	if h.tlsEnabled {
		caCert, err := ioutil.ReadFile(h.trustStoreFilePath)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:            caCertPool,
					InsecureSkipVerify: true,
				},
			},
		}
	} else {
		client = &http.Client{}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Println("[DEBUG] response Status:", resp.Status)
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	return nil
}
