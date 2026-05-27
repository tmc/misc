package webkithost

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/tmc/apple/corefoundation"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/security"
	"strings"
)

type keychainQuery struct {
	Service        string `json:"service"`
	Account        string `json:"account"`
	Label          string `json:"label"`
	CertificateDER string `json:"certificate_der"`
	CertificatePEM string `json:"certificate_pem"`
	Hostname       string `json:"hostname"`
	Policy         string `json:"policy"`
}

type keychainResult struct {
	Class       string `json:"class"`
	Service     string `json:"service,omitempty"`
	Account     string `json:"account,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Status      int32  `json:"status"`
}

type trustResult struct {
	Trusted     bool   `json:"trusted"`
	Policy      string `json:"policy"`
	Hostname    string `json:"hostname,omitempty"`
	Result      string `json:"result"`
	ResultCode  int32  `json:"result_code"`
	Error       string `json:"error,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

func (h *Host) applyKeychainSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch verb {
	case "query generic-password":
		return h.queryGenericPassword(id)
	case "query certificate":
		return h.queryCertificate(id)
	case "trust-evaluate":
		return h.evaluateTrust(id)
	default:
		return nil
	}
}

func (h *Host) evaluateTrust(id string) error {
	var q keychainQuery
	if err := json.Unmarshal([]byte(readAppleFSString(h, "keychain/"+id+"/query")), &q); err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror parse query: "+err.Error()+"\n"))
		return fmt.Errorf("parse query: %w", err)
	}
	result, err := evaluateCertificateTrust(q)
	if err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	data, err := json.MarshalIndent([]trustResult{result}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := h.appleFS.WriteFile("keychain/"+id+"/results", data); err != nil {
		return err
	}
	state := "untrusted"
	if result.Trusted {
		state = "trusted"
	}
	return h.appleFS.WriteFile("keychain/"+id+"/status", []byte(fmt.Sprintf("status %s\ntrust-result %s\n", state, result.Result)))
}

func (h *Host) queryGenericPassword(id string) error {
	var q keychainQuery
	if err := json.Unmarshal([]byte(readAppleFSString(h, "keychain/"+id+"/query")), &q); err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror parse query: "+err.Error()+"\n"))
		return fmt.Errorf("parse query: %w", err)
	}
	query, err := genericPasswordQuery(q)
	if err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	var result corefoundation.CFTypeRef
	status := security.SecItemCopyMatching(query, &result)
	if result != nil {
		defer corefoundation.CFRelease(result)
	}
	records := []keychainResult{}
	if status == 0 && result != nil {
		records = append(records, keychainResult{
			Class:       "generic-password",
			Service:     q.Service,
			Account:     q.Account,
			Description: describeCFType(result),
			Status:      status,
		})
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := h.appleFS.WriteFile("keychain/"+id+"/results", data); err != nil {
		return err
	}
	state := "done"
	if status != 0 {
		state = "no-match"
	}
	return h.appleFS.WriteFile("keychain/"+id+"/status", []byte(fmt.Sprintf("status %s\nosstatus %d\n", state, status)))
}

func (h *Host) queryCertificate(id string) error {
	var q keychainQuery
	if err := json.Unmarshal([]byte(readAppleFSString(h, "keychain/"+id+"/query")), &q); err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror parse query: "+err.Error()+"\n"))
		return fmt.Errorf("parse query: %w", err)
	}
	query, err := certificateQuery(q)
	if err != nil {
		_ = h.appleFS.WriteFile("keychain/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	var result corefoundation.CFTypeRef
	status := security.SecItemCopyMatching(query, &result)
	if result != nil {
		defer corefoundation.CFRelease(result)
	}
	records := []keychainResult{}
	if status == 0 && result != nil {
		records = append(records, keychainResult{
			Class:       "certificate",
			Label:       q.Label,
			Description: describeCFType(result),
			Status:      status,
		})
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := h.appleFS.WriteFile("keychain/"+id+"/results", data); err != nil {
		return err
	}
	state := "done"
	if status != 0 {
		state = "no-match"
	}
	return h.appleFS.WriteFile("keychain/"+id+"/status", []byte(fmt.Sprintf("status %s\nosstatus %d\n", state, status)))
}

func genericPasswordQuery(q keychainQuery) (corefoundation.CFDictionaryRef, error) {
	for name, value := range map[string]string{
		"kSecClass":                security.KSecClass,
		"kSecClassGenericPassword": security.KSecClassGenericPassword,
		"kSecAttrService":          security.KSecAttrService,
		"kSecAttrAccount":          security.KSecAttrAccount,
		"kSecReturnAttributes":     security.KSecReturnAttributes,
		"kSecMatchLimit":           security.KSecMatchLimit,
		"kSecMatchLimitOne":        security.KSecMatchLimitOne,
	} {
		if value == "" {
			return 0, fmt.Errorf("security constant %s unavailable", name)
		}
	}
	query := foundation.NewMutableDictionaryWithCapacity(5)
	query.SetObjectForKey(foundation.NewStringWithString(security.KSecClassGenericPassword), foundation.NewStringWithString(security.KSecClass))
	if q.Service != "" {
		query.SetObjectForKey(foundation.NewStringWithString(q.Service), foundation.NewStringWithString(security.KSecAttrService))
	}
	if q.Account != "" {
		query.SetObjectForKey(foundation.NewStringWithString(q.Account), foundation.NewStringWithString(security.KSecAttrAccount))
	}
	query.SetObjectForKey(foundation.NewNumberWithBool(true), foundation.NewStringWithString(security.KSecReturnAttributes))
	query.SetObjectForKey(foundation.NewStringWithString(security.KSecMatchLimitOne), foundation.NewStringWithString(security.KSecMatchLimit))
	return corefoundation.CFDictionaryRef(query.GetID()), nil
}

func certificateQuery(q keychainQuery) (corefoundation.CFDictionaryRef, error) {
	for name, value := range map[string]string{
		"kSecClass":            security.KSecClass,
		"kSecClassCertificate": security.KSecClassCertificate,
		"kSecAttrLabel":        security.KSecAttrLabel,
		"kSecReturnAttributes": security.KSecReturnAttributes,
		"kSecMatchLimit":       security.KSecMatchLimit,
		"kSecMatchLimitOne":    security.KSecMatchLimitOne,
	} {
		if value == "" {
			return 0, fmt.Errorf("security constant %s unavailable", name)
		}
	}
	query := foundation.NewMutableDictionaryWithCapacity(5)
	query.SetObjectForKey(foundation.NewStringWithString(security.KSecClassCertificate), foundation.NewStringWithString(security.KSecClass))
	if q.Label != "" {
		query.SetObjectForKey(foundation.NewStringWithString(q.Label), foundation.NewStringWithString(security.KSecAttrLabel))
	}
	query.SetObjectForKey(foundation.NewNumberWithBool(true), foundation.NewStringWithString(security.KSecReturnAttributes))
	query.SetObjectForKey(foundation.NewStringWithString(security.KSecMatchLimitOne), foundation.NewStringWithString(security.KSecMatchLimit))
	return corefoundation.CFDictionaryRef(query.GetID()), nil
}

func evaluateCertificateTrust(q keychainQuery) (trustResult, error) {
	der, summary, err := certificateDER(q)
	if err != nil {
		return trustResult{}, err
	}
	certData := foundation.NewDataFromBytes(der)
	cert := security.SecCertificateCreateWithData(corefoundation.KCFAllocatorDefault, corefoundation.CFDataRef(certData.GetID()))
	if cert == 0 {
		return trustResult{}, fmt.Errorf("create certificate")
	}
	defer corefoundation.CFRelease(cfPtr(cert))

	policy, policyName, err := trustPolicy(q)
	if err != nil {
		return trustResult{}, err
	}
	if policy == 0 {
		return trustResult{}, fmt.Errorf("create policy")
	}
	defer corefoundation.CFRelease(cfPtr(policy))

	var trust security.SecTrustRef
	status := security.SecTrustCreateWithCertificates(cfPtr(cert), cfPtr(policy), &trust)
	if status != 0 {
		return trustResult{}, fmt.Errorf("create trust: osstatus %d", status)
	}
	if trust == 0 {
		return trustResult{}, fmt.Errorf("create trust")
	}
	defer corefoundation.CFRelease(cfPtr(trust))

	var cferr corefoundation.CFErrorRef
	trusted := security.SecTrustEvaluateWithError(trust, &cferr)
	errText := ""
	if cferr != 0 {
		errText = describeCFType(cfPtr(cferr))
		corefoundation.CFRelease(cfPtr(cferr))
	}
	var trustType security.SecTrustResultType
	resultStatus := security.SecTrustGetTrustResult(trust, &trustType)
	if resultStatus != 0 {
		return trustResult{}, fmt.Errorf("get trust result: osstatus %d", resultStatus)
	}
	return trustResult{
		Trusted:     trusted,
		Policy:      policyName,
		Hostname:    q.Hostname,
		Result:      trustType.String(),
		ResultCode:  int32(trustType),
		Error:       errText,
		Certificate: summary,
	}, nil
}

func certificateDER(q keychainQuery) ([]byte, string, error) {
	switch {
	case strings.TrimSpace(q.CertificateDER) != "":
		der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(q.CertificateDER))
		if err != nil {
			return nil, "", fmt.Errorf("decode certificate_der: %w", err)
		}
		return der, certificateSummary(der), nil
	case strings.TrimSpace(q.CertificatePEM) != "":
		block, _ := pem.Decode([]byte(q.CertificatePEM))
		if block == nil {
			return nil, "", fmt.Errorf("decode certificate_pem")
		}
		if block.Type != "CERTIFICATE" {
			return nil, "", fmt.Errorf("decode certificate_pem: got %s", block.Type)
		}
		return block.Bytes, certificateSummary(block.Bytes), nil
	default:
		return nil, "", fmt.Errorf("missing certificate_pem or certificate_der")
	}
}

func certificateSummary(der []byte) string {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return ""
	}
	return cert.Subject.String()
}

func trustPolicy(q keychainQuery) (security.SecPolicyRef, string, error) {
	policy := strings.TrimSpace(q.Policy)
	if policy == "" {
		if q.Hostname != "" {
			policy = "ssl-server"
		} else {
			policy = "basic-x509"
		}
	}
	switch policy {
	case "basic-x509", "x509":
		return security.SecPolicyCreateBasicX509(), "basic-x509", nil
	case "ssl-server", "ssl":
		var host corefoundation.CFStringRef
		if q.Hostname != "" {
			host = cfString(q.Hostname)
			defer corefoundation.CFRelease(cfPtr(host))
		}
		return security.SecPolicyCreateSSL(true, host), "ssl-server", nil
	case "ssl-client":
		return security.SecPolicyCreateSSL(false, 0), "ssl-client", nil
	default:
		return 0, "", fmt.Errorf("unknown policy %q", policy)
	}
}

func describeCFType(v corefoundation.CFTypeRef) string {
	desc := corefoundation.CFCopyDescription(v)
	if desc == 0 {
		return ""
	}
	defer corefoundation.CFRelease(cfPtr(desc))
	return cfStringValue(desc)
}
