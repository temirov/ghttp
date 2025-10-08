package certificates

const (
	// DefaultCertificateAuthorityCommonName names the development root certificate authority.
	DefaultCertificateAuthorityCommonName = "ghttp Development CA"
	// DefaultCertificateAuthorityOrganizationalUnit identifies the default organizational unit.
	DefaultCertificateAuthorityOrganizationalUnit = "ghttp"
	// DefaultCertificateAuthorityOrganization identifies the default organization.
	DefaultCertificateAuthorityOrganization = "temirov"
	// DefaultCertificateDirectoryName is the directory containing certificate material.
	DefaultCertificateDirectoryName = "certs"
	// DefaultRootCertificateFileName is the filename for the root certificate.
	DefaultRootCertificateFileName = "ca.pem"
	// DefaultRootPrivateKeyFileName is the filename for the root private key.
	DefaultRootPrivateKeyFileName = "ca.key"
	// DefaultLeafCertificateFileName is the filename for the issued leaf certificate.
	DefaultLeafCertificateFileName = "localhost.pem"
	// DefaultLeafPrivateKeyFileName is the filename for the issued leaf private key.
	DefaultLeafPrivateKeyFileName = "localhost.key"
)
