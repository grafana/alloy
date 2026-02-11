package vault

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/syntax/alloytypes"
	vault "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/hashicorp/vault/api/auth/aws"
	"github.com/hashicorp/vault/api/auth/azure"
	"github.com/hashicorp/vault/api/auth/gcp"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/hashicorp/vault/api/auth/ldap"
	"github.com/hashicorp/vault/api/auth/userpass"
)

// An authMethod can configure a Vault client to be authenticated using a
// specific authentication method.
//
// The vaultAuthenticate method will be called each time a new token is needed
// (e.g., if renewal failed). vaultAuthenticate method may return a nil secret
// if the authentication method does not generate a secret.
type authMethod interface {
	vaultAuthenticate(context.Context, *vault.Client) (*vault.Secret, error)
}

// AuthArguments defines a single authenticationstring type in a remote.vault
// component instance. These are embedded as an enum field so only one may be
// set per AuthArguments.
type AuthArguments struct {
	AuthToken      *AuthToken      `alloy:"token,block,optional"`
	AuthAppRole    *AuthAppRole    `alloy:"approle,block,optional"`
	AuthAWS        *AuthAWS        `alloy:"aws,block,optional"`
	AuthAzure      *AuthAzure      `alloy:"azure,block,optional"`
	AuthGCP        *AuthGCP        `alloy:"gcp,block,optional"`
	AuthKubernetes *AuthKubernetes `alloy:"kubernetes,block,optional"`
	AuthLDAP       *AuthLDAP       `alloy:"ldap,block,optional"`
	AuthUserPass   *AuthUserPass   `alloy:"userpass,block,optional"`
	AuthCustom     *AuthCustom     `alloy:"custom,block,optional"`
}

func (a *AuthArguments) authMethod() authMethod {
	switch {
	case a.AuthToken != nil:
		return a.AuthToken
	case a.AuthAppRole != nil:
		return a.AuthAppRole
	case a.AuthAWS != nil:
		return a.AuthAWS
	case a.AuthAzure != nil:
		return a.AuthAzure
	case a.AuthGCP != nil:
		return a.AuthGCP
	case a.AuthKubernetes != nil:
		return a.AuthKubernetes
	case a.AuthLDAP != nil:
		return a.AuthLDAP
	case a.AuthUserPass != nil:
		return a.AuthUserPass
	case a.AuthCustom != nil:
		return a.AuthCustom
	}

	// Return a default authMethod which always fails. This code should not be
	// reachable unless this function was mistakenly not updated after
	// implemeneting a new auth block.
	return invalidAuth{}
}

// AuthToken authenticates against Vault with a token.
type AuthToken struct {
	Token alloytypes.Secret `alloy:"token,attr"`
}

func (a *AuthToken) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	cli.SetToken(string(a.Token))
	return nil, nil
}

// AuthAppRole authenticates against Vault with AppRole.
type AuthAppRole struct {
	RoleID        string            `alloy:"role_id,attr"`
	Secret        alloytypes.Secret `alloy:"secret,attr"`
	WrappingToken bool              `alloy:"wrapping_token,attr,optional"`
	MountPath     string            `alloy:"mount_path,attr,optional"`
}

// DefaultAuthAppRole provides default settings for AuthAppRole.
var DefaultAuthAppRole = AuthAppRole{
	MountPath: "approle",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthAppRole) SetToDefault() {
	*a = DefaultAuthAppRole
}

func (a *AuthAppRole) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	secret := &approle.SecretID{FromString: string(a.Secret)}

	var opts []approle.LoginOption
	if a.WrappingToken {
		opts = append(opts, approle.WithWrappingToken())
	}
	if a.MountPath != "" {
		opts = append(opts, approle.WithMountPath(a.MountPath))
	}

	auth, err := approle.NewAppRoleAuth(a.RoleID, secret, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.approle: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.approle: %w", err)
	}
	return s, nil
}

// AuthAWS authenticates against Vault with AWS.
type AuthAWS struct {
	// Type specifies the mechanism used to authenticate with AWS. Should be
	// either ec2 or iam.
	Type              string `alloy:"type,attr"`
	Region            string `alloy:"region,attr,optional"`
	Role              string `alloy:"role,attr,optional"`
	IAMServerIDHeader string `alloy:"iam_server_id_header,attr,optional"`
	// EC2SignatureType specifies the signature to use against EC2. Only used
	// when Type is ec2. Valid options are identity and pkcs7 (default).
	EC2SignatureType string `alloy:"ec2_signature_type,attr,optional"`
	MountPath        string `alloy:"mount_path,attr,optional"`
}

const (
	authAWSTypeEC2 = "ec2"
	authAWSTypeIAM = "iam"
)

// DefaultAuthAWS provides default settings for AuthAWS.
var DefaultAuthAWS = AuthAWS{
	MountPath:        "aws",
	Type:             authAWSTypeIAM,
	Region:           "us-east-1",
	EC2SignatureType: "pkcs7",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthAWS) SetToDefault() {
	*a = DefaultAuthAWS
}

// Validate implements syntax.Validator.
func (a *AuthAWS) Validate() error {
	switch a.Type {
	case "":
		return fmt.Errorf("type must not be empty")
	case authAWSTypeEC2, authAWSTypeIAM:
		// no-op
	default:
		return fmt.Errorf("unrecognized type %q, expected one of ec2,iam", a.Type)
	}

	switch a.EC2SignatureType {
	case "":
		return fmt.Errorf("ec2_signature_type must not be empty")
	case "pkcs7", "identity":
		// no-op
	default:
		return fmt.Errorf(": unrecognized ec2_signature_type %q, expected one of pkcs7,identity", a.Type)
	}

	return nil
}

func (a *AuthAWS) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	// Re-validate for safety.
	if err := a.Validate(); err != nil {
		return nil, err
	}

	var opts []aws.LoginOption

	switch a.Type {
	case authAWSTypeEC2:
		opts = append(opts, aws.WithEC2Auth())
	case authAWSTypeIAM:
		opts = append(opts, aws.WithIAMAuth())
	}
	if a.Region != "" {
		opts = append(opts, aws.WithRegion(a.Region))
	}
	if a.Role != "" {
		opts = append(opts, aws.WithRole(a.Role))
	}
	if a.IAMServerIDHeader != "" {
		opts = append(opts, aws.WithIAMServerIDHeader(a.IAMServerIDHeader))
	}
	switch a.EC2SignatureType {
	case "", "pkcs7":
		opts = append(opts, aws.WithPKCS7Signature())
	case "identity":
		opts = append(opts, aws.WithIdentitySignature())
	}
	if a.MountPath != "" {
		opts = append(opts, aws.WithMountPath(a.MountPath))
	}

	auth, err := aws.NewAWSAuth(opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.aws: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.aws: %w", err)
	}
	return s, nil
}

// AuthAzure authenticates against Vault with Azure.
type AuthAzure struct {
	Role        string `alloy:"role,attr"`
	ResourceURL string `alloy:"resource_url,attr,optional"`
	MountPath   string `alloy:"mount_path,attr,optional"`
}

// DefaultAuthAzure provides default settings for AuthAzure.
var DefaultAuthAzure = AuthAzure{
	MountPath:   "azure",
	ResourceURL: "https://management.azure.com/",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthAzure) SetToDefault() {
	*a = DefaultAuthAzure
}

func (a *AuthAzure) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	var opts []azure.LoginOption

	if a.ResourceURL != "" {
		opts = append(opts, azure.WithResource(a.ResourceURL))
	}
	if a.MountPath != "" {
		opts = append(opts, azure.WithMountPath(a.MountPath))
	}

	auth, err := azure.NewAzureAuth(a.Role, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.azure: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.azure: %w", err)
	}
	return s, nil
}

// AuthGCP authenticates against Vault with GCP.
type AuthGCP struct {
	Role string `alloy:"role,attr"`
	// Type specifies the mechanism used to authenticate with GCS. Should be
	// either gce or iam.
	Type              string `alloy:"type,attr"`
	IAMServiceAccount string `alloy:"iam_service_account,attr,optional"`
	MountPath         string `alloy:"mount_path,attr,optional"`
}

const (
	authGCPTypeGCE = "gce"
	authGCPTypeIAM = "iam"
)

// DefaultAuthGCP provides default settings for AuthGCP.
var DefaultAuthGCP = AuthGCP{
	MountPath: "gcp",
	Type:      authGCPTypeGCE,
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthGCP) SetToDefault() {
	*a = DefaultAuthGCP
}

// Validate implements syntax.Validator.
func (a *AuthGCP) Validate() error {
	switch a.Type {
	case authGCPTypeGCE:
		// no-op
	case authGCPTypeIAM:
		if a.IAMServiceAccount == "" {
			return fmt.Errorf("iam_service_account must be provided when type is iam")
		}
	default:
		return fmt.Errorf("unrecognized type %q, expected one of gce,iam", a.Type)
	}

	return nil
}

func (a *AuthGCP) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	// Re-validate for safety.
	if err := a.Validate(); err != nil {
		return nil, err
	}

	var opts []gcp.LoginOption

	switch a.Type {
	case authGCPTypeGCE:
		opts = append(opts, gcp.WithGCEAuth())
	case authGCPTypeIAM:
		opts = append(opts, gcp.WithIAMAuth(a.IAMServiceAccount))
	}

	if a.MountPath != "" {
		opts = append(opts, gcp.WithMountPath(a.MountPath))
	}

	auth, err := gcp.NewGCPAuth(a.Role, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.gcp: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.gcp: %w", err)
	}
	return s, nil
}

// AuthKubernetes authenticates against Vault with Kubernetes.
type AuthKubernetes struct {
	Role                    string `alloy:"role,attr"`
	ServiceAccountTokenFile string `alloy:"service_account_file,attr,optional"`
	MountPath               string `alloy:"mount_path,attr,optional"`
}

// DefaultAuthKubernetes provides default settings for AuthKubernetes.
var DefaultAuthKubernetes = AuthKubernetes{
	MountPath:               "kubernetes",
	ServiceAccountTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthKubernetes) SetToDefault() {
	*a = DefaultAuthKubernetes
}

func (a *AuthKubernetes) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	var opts []kubernetes.LoginOption

	if a.ServiceAccountTokenFile != "" {
		opts = append(opts, kubernetes.WithServiceAccountTokenPath(a.ServiceAccountTokenFile))
	}
	if a.MountPath != "" {
		opts = append(opts, kubernetes.WithMountPath(a.MountPath))
	}

	auth, err := kubernetes.NewKubernetesAuth(a.Role, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.kubernetes: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.kubernetes: %w", err)
	}
	return s, nil
}

// AuthLDAP authenticates against Vault with LDAP.
type AuthLDAP struct {
	Username  string            `alloy:"username,attr"`
	Password  alloytypes.Secret `alloy:"password,attr"`
	MountPath string            `alloy:"mount_path,attr,optional"`
}

// DefaultAuthLDAP provides default settings for AuthLDAP.
var DefaultAuthLDAP = AuthLDAP{
	MountPath: "ldap",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthLDAP) SetToDefault() {
	*a = DefaultAuthLDAP
}

func (a *AuthLDAP) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	secret := &ldap.Password{FromString: string(a.Password)}

	var opts []ldap.LoginOption

	if a.MountPath != "" {
		opts = append(opts, ldap.WithMountPath(a.MountPath))
	}

	auth, err := ldap.NewLDAPAuth(a.Username, secret, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.ldap: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.ldap: %w", err)
	}
	return s, nil
}

// AuthUserPass authenticates against Vault with a username and password.
type AuthUserPass struct {
	Username  string            `alloy:"username,attr"`
	Password  alloytypes.Secret `alloy:"password,attr"`
	MountPath string            `alloy:"mount_path,attr,optional"`
}

// DefaultAuthUserPass provides default settings for AuthUserPass.
var DefaultAuthUserPass = AuthUserPass{
	MountPath: "userpass",
}

// SetToDefault implements syntax.Defaulter.
func (a *AuthUserPass) SetToDefault() {
	*a = DefaultAuthUserPass
}

func (a *AuthUserPass) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	secret := &userpass.Password{FromString: string(a.Password)}

	var opts []userpass.LoginOption

	if a.MountPath != "" {
		opts = append(opts, userpass.WithMountPath(a.MountPath))
	}

	auth, err := userpass.NewUserpassAuth(a.Username, secret, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth.userpass: %w", err)
	}
	s, err := cli.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("auth.userpass: %w", err)
	}
	return s, nil
}

// AuthCustom provides a custom authentication method.
type AuthCustom struct {
	// Path to use for logging in (e.g., auth/kubernetes/login, etc.)
	Path      string                       `alloy:"path,attr"`
	Data      map[string]alloytypes.Secret `alloy:"data,attr"`
	Namespace string                       `alloy:"namespace,attr,optional"`
}

// Login implements vault.AuthMethod.
func (a *AuthCustom) Login(ctx context.Context, client *vault.Client) (*vault.Secret, error) {
	data := make(map[string]any, len(a.Data))
	for k, v := range a.Data {
		data[k] = string(v)
	}
	return client.Logical().WriteWithContext(ctx, a.Path, data)
}

func (a *AuthCustom) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	var s *vault.Secret
	var err error

	if a.Namespace != "" {
		// Authenticating to a namespace different from the one used by the KV store is likely applicable to other forms of authentication on enterprise vault
		// but only testable with authCustom when added in https://github.com/grafana/alloy/pull/2945
		s, err = cli.WithNamespace(a.Namespace).Auth().Login(ctx, a)
		if err == nil {
			// As the WithNameSpace function creates a shallow copy of the original client, we need to set the acquired
			// token to the original client manually.
			cli.SetToken(s.Auth.ClientToken)
		}
	} else {
		s, err = cli.Auth().Login(ctx, a)
	}

	if err != nil {
		return nil, fmt.Errorf("auth.custom: %w", err)
	}
	return s, nil
}

type invalidAuth struct {
	Name string
}

func (a invalidAuth) vaultAuthenticate(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	return nil, fmt.Errorf("unsupported auth method configured")
}
