package webkithost

import "testing"

func TestGenericPasswordQueryBuilds(t *testing.T) {
	query, err := genericPasswordQuery(keychainQuery{
		Service: "com.example",
		Account: "me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if query == 0 {
		t.Fatal("query is nil")
	}
}

func TestKeychainCertificateQuery(t *testing.T) {
	query, err := certificateQuery(keychainQuery{Label: "Developer ID"})
	if err != nil {
		t.Fatal(err)
	}
	if query == 0 {
		t.Fatal("certificateQuery returned nil")
	}
}

func TestCertificateDERRequiresInput(t *testing.T) {
	if _, _, err := certificateDER(keychainQuery{}); err == nil {
		t.Fatal("certificateDER succeeded without input")
	}
}

func TestCertificateDERFromPEM(t *testing.T) {
	const cert = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIRAM1Lo2SgvWcZVNQ8BYmWqP0wCgYIKoZIzj0EAwIw
EjEQMA4GA1UEAxMHV2FuaXggQ0EwHhcNMjYwMTAxMDAwMDAwWhcNMjcwMTAx
MDAwMDAwWjASMRAwDgYDVQQDEwdXYW5peCBDQTBZMBMGByqGSM49AgEGCCqG
SM49AwEHA0IABM6A7bim5OwHjGe/fdb6qvwggXdd4/bJ6vliQ4L5Mkkz3rgr
sVEjZdb17YALgTETgVYBzaArqMnKoSMRAA5ykE6jYzBhMA4GA1UdDwEB/wQE
AwICpDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQBHfv1h9N90LzIwEvy
CLi0zS5W7zAfBgNVHSMEGDAWgBQBHfv1h9N90LzIwEvyCLi0zS5W7zAKBggq
hkjOPQQDAgNIADBFAiEAoeVq5FzdfpxuCEwvMdm0Aw7AUiTfWLPpTQOaJB1o
wX4CICPRD0oVhC1HE0cV1wZyNkN9rJwbgwKjEFlq6Sp1lhFO
-----END CERTIFICATE-----`
	der, _, err := certificateDER(keychainQuery{CertificatePEM: cert})
	if err != nil {
		t.Fatal(err)
	}
	if len(der) == 0 {
		t.Fatal("empty der")
	}
}

func TestTrustPolicyRejectsUnknown(t *testing.T) {
	if _, _, err := trustPolicy(keychainQuery{Policy: "unknown"}); err == nil {
		t.Fatal("trustPolicy accepted unknown policy")
	}
}
