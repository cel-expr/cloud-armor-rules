// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudarmor

import (
	"strings"

	"github.com/google/cel-go/interpreter"
	"gopkg.in/yaml.v3"
)

// Variables serves as a container for all of the variables that are available to the Cloud Armor
// expression.
type Variables struct {
	Request *Request `yaml:"request"`
	Origin  *Origin  `yaml:"origin"`
	Token   *Token   `yaml:"token"`
}

// VariablesFromYAML converts a YAML representation of the variables to a Variables type.
//
// The YAML representation is expected to be a map of variable names to values. The variable names
// are expected to match the names that are defined in the Cloud Armor expression language.
//
// The return value is the Variables type or an error if the YAML is invalid.
func VariablesFromYAML(yamlBytes []byte) (*Variables, error) {
	v := &Variables{}
	if err := yaml.Unmarshal(yamlBytes, v); err != nil {
		return nil, err
	}
	return SafeVariables(v), nil
}

// SafeVariables ensures that all of the variables are initialized to their default values.
func SafeVariables(v *Variables) *Variables {
	if v.Request == nil {
		v.Request = &Request{}
	}
	if v.Request.Headers == nil {
		v.Request.Headers = make(Headers)
	}
	for k, val := range v.Request.Headers {
		v.Request.Headers[strings.ToLower(k)] = val
	}
	if v.Origin == nil {
		v.Origin = &Origin{}
	}
	if v.Token == nil {
		v.Token = &Token{}
	}
	if v.Token.RecaptchaExemption == nil {
		v.Token.RecaptchaExemption = &RecaptchaExemption{}
	}
	if v.Token.RecaptchaAction == nil {
		v.Token.RecaptchaAction = &RecaptchaAction{}
	}
	if v.Token.RecaptchaSession == nil {
		v.Token.RecaptchaSession = &RecaptchaSession{}
	}
	return v
}

// ResolveName resolves the given name to a value in the variables container.
//
// The name is expected to be in the format of the variables that are defined in the Cloud Armor
// language. For example, "request.method" or "token.recaptcha_action.score".
//
// The return value is the resolved value and a boolean indicating if the name was resolved.
func (v *Variables) ResolveName(name string) (any, bool) {
	switch name {
	case "request.method":
		return v.Request.Method, true
	case "request.headers":
		return v.Request.Headers, true
	case "request.path":
		return v.Request.Path, true
	case "request.query":
		return v.Request.Query, true
	case "request.scheme":
		return v.Request.Scheme, true
	case "origin.ip":
		return v.Origin.IP, true
	case "origin.region_code":
		return v.Origin.RegionCode, true
	case "origin.asn":
		return v.Origin.ASN, true
	case "origin.user_ip":
		return v.Origin.UserIP, true
	case "origin.tls_ja3_fingerprint":
		return v.Origin.TLSJA3Fingerprint, true
	case "origin.tls_ja4_fingerprint":
		return v.Origin.TLSJA4Fingerprint, true
	case "token.recaptcha_exemption.valid":
		return v.Token.RecaptchaExemption.Valid, true
	case "token.recaptcha_action.score":
		return v.Token.RecaptchaAction.Score, true
	case "token.recaptcha_action.captcha_status":
		return v.Token.RecaptchaAction.CaptchaStatus, true
	case "token.recaptcha_action.action":
		return v.Token.RecaptchaAction.Action, true
	case "token.recaptcha_action.valid":
		return v.Token.RecaptchaAction.Valid, true
	case "token.recaptcha_session.score":
		return v.Token.RecaptchaSession.Score, true
	case "token.recaptcha_session.valid":
		return v.Token.RecaptchaSession.Valid, true
	default:
		return nil, false
	}
}

// Parent returns nil as hierarchical context building is not supported within Cloud Armor.
func (v *Variables) Parent() interpreter.Activation {
	return nil
}

// Headers represents a map of HTTP headers.
type Headers map[string]string

// HTTPHeaders converts a map of headers to a Headers type.
//
// The keys are converted to lower case to match the behavior of the Cloud Armor expression
// language.
func HTTPHeaders(headers map[string]string) Headers {
	for k, v := range headers {
		headers[strings.ToLower(k)] = v
	}
	return Headers(headers)
}

// Token represents the token attributes available to the Cloud Armor expression.
type Token struct {
	RecaptchaExemption *RecaptchaExemption `yaml:"recaptcha_exemption"`
	RecaptchaAction    *RecaptchaAction    `yaml:"recaptcha_action"`
	RecaptchaSession   *RecaptchaSession   `yaml:"recaptcha_session"`
}

// Request represents the request attributes available to the Cloud Armor expression.
type Request struct {
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Path    string            `yaml:"path"`
	Query   string            `yaml:"query"`
	Scheme  string            `yaml:"scheme"`
}

// Origin represents the origin attributes available to the Cloud Armor expression.
type Origin struct {
	IP                string `yaml:"ip"`
	RegionCode        string `yaml:"region_code"`
	ASN               int64  `yaml:"asn"`
	UserIP            string `yaml:"user_ip"`
	TLSJA3Fingerprint string `yaml:"tls_ja3_fingerprint"`
	TLSJA4Fingerprint string `yaml:"tls_ja4_fingerprint"`
}

// RecaptchaExemption represents the reCaptcha exemption attributes available to the Cloud Armor expression.
type RecaptchaExemption struct {
	Valid bool `yaml:"valid"`
}

// RecaptchaAction represents the reCaptcha action attributes available to the Cloud Armor expression.
type RecaptchaAction struct {
	Score         float64 `yaml:"score"`
	CaptchaStatus string  `yaml:"captcha_status"`
	Action        string  `yaml:"action"`
	Valid         bool    `yaml:"valid"`
}

// RecaptchaSession represents the reCaptcha session attributes available to the Cloud Armor expression.
type RecaptchaSession struct {
	Score float64 `yaml:"score"`
	Valid bool    `yaml:"valid"`
}
