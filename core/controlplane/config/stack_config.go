package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/coreos/userdatavalidation"
	"github.com/kubernetes-incubator/kube-aws/filereader/jsontemplate"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"github.com/kubernetes-incubator/kube-aws/model"
)

type StackConfig struct {
	*Config
	StackTemplateOptions
	UserDataWorker        string
	UserDataController    string
	UserDataEtcd          string
	ControllerSubnetIndex int
}

type CompressedStackConfig struct {
	*StackConfig
}

// UserDataControllerS3Prefix is the prefix prepended to all userdata-controller-<fingerprint> files uploaded to S3
// Use this to author the IAM policy to provide controller nodes least required permissions for getting the files from S3
func (c *StackConfig) UserDataControllerS3Prefix() (string, error) {
	s3dir, err := c.userDataControllerS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataControllerS3Prefix : %v", err)
	}
	return fmt.Sprintf("%s/userdata-controller", s3dir), nil
}

func (c *StackConfig) userDataControllerS3Directory() (string, error) {
	s3uri, err := url.Parse(c.ClusterExportedStacksS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in userDataControllerS3Directory : %v", err)
	}
	return fmt.Sprintf("%s%s/%s", s3uri.Host, s3uri.Path, c.StackName()), nil
}

// UserDataControllerS3URI is the URI to an userdata-controller-<fingerprint> file used to provision controller nodes
// Use this to run download the file by running e.g. `aws cp *return value of UserDataControllerS3URI* ./`
func (c *StackConfig) UserDataControllerS3URI() (string, error) {
	s3dir, err := c.userDataControllerS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataControllerS3URI : %v", err)
	}
	return fmt.Sprintf("s3://%s/%s", s3dir, c.UserDataControllerFileName()), nil
}

// UserDataControllerFileName is used to upload and download userdata-controller-<fingerprint> files
func (c *StackConfig) UserDataControllerFileName() string {
	return "userdata-controller-" + fingerprint.SHA256(c.UserDataController)
}

// UserDataEtcdS3Prefix is the prefix prepended to all userdata-etcd-<fingerprint> files uploaded to S3
// Use this to author the IAM policy to provide etcd nodes least required permissions for getting the files from S3
func (c *StackConfig) UserDataEtcdS3Prefix() (string, error) {
	s3dir, err := c.userDataEtcdS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataEtcdS3Prefix : %v", err)
	}
	return fmt.Sprintf("%s/userdata-etcd", s3dir), nil
}

func (c *StackConfig) userDataEtcdS3Directory() (string, error) {
	s3uri, err := url.Parse(c.ClusterExportedStacksS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in userDataEtcdS3Directory : %v", err)
	}
	return fmt.Sprintf("%s%s/%s", s3uri.Host, s3uri.Path, c.StackName()), nil
}

// UserDataEtcdS3URI is the URI to an userdata-etcd-<fingerprint> file used to provision etcd nodes
// Use this to run download the file by running e.g. `aws cp *return value of UserDataEtcdS3URI* ./`
func (c *StackConfig) UserDataEtcdS3URI() (string, error) {
	s3dir, err := c.userDataEtcdS3Directory()
	if err != nil {
		return "", fmt.Errorf("Error in UserDataEtcdS3URI : %v", err)
	}
	return fmt.Sprintf("s3://%s/%s", s3dir, c.UserDataEtcdFileName()), nil
}

// UserDataEtcdFileName is used to upload and download userdata-etcd-<fingerprint> files
func (c *StackConfig) UserDataEtcdFileName() string {
	return "userdata-etcd-" + fingerprint.SHA256(c.UserDataEtcd)
}

func (c *StackConfig) s3Folders() model.S3Folders {
	return model.NewS3Folders(c.S3URI, c.ClusterName)
}

func (c *StackConfig) ClusterS3URI() string {
	return c.s3Folders().Cluster().URI()
}

func (c *StackConfig) ClusterExportedStacksS3URI() string {
	return c.s3Folders().ClusterExportedStacks().URI()
}

// EtcdSnapshotsS3Path is a pair of a S3 bucket and a key of an S3 object containing an etcd cluster snapshot
func (c *StackConfig) EtcdSnapshotsS3PathRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3PathRef : %v", err)
	}
	return fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, s3uri.Host, s3uri.Path), nil
}

func (c *StackConfig) EtcdSnapshotsS3Bucket() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Bucket : %v", err)
	}
	return s3uri.Host, nil
}

func (c *StackConfig) EtcdSnapshotsS3PrefixRef() (string, error) {
	s3uri, err := url.Parse(c.ClusterS3URI())
	if err != nil {
		return "", fmt.Errorf("Error in EtcdSnapshotsS3Prefix : %v", err)
	}
	s3path := fmt.Sprintf(`{ "Fn::Join" : [ "", [ "%s/instances/", { "Fn::Select" : [ "2", { "Fn::Split": [ "/", { "Ref": "AWS::StackId" }]} ]}, "/etcd-snapshots" ]]}`, s3uri.Path)
	return strings.TrimLeft(s3path, "/"), nil
}

func (c *StackConfig) ValidateUserData() error {
	err := userdatavalidation.Execute([]userdatavalidation.Entry{
		{Name: "UserDataWorker", Content: c.UserDataWorker},
		{Name: "UserDataController", Content: c.UserDataController},
		{Name: "UserDataEtcd", Content: c.UserDataEtcd},
	})

	return err
}

func (c *StackConfig) Compress() (*CompressedStackConfig, error) {
	var stackConfig CompressedStackConfig
	stackConfig.StackConfig = &(*c)
	return &stackConfig, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsBytes() ([]byte, error) {
	bytes, err := jsontemplate.GetBytes(c.StackTemplateTmplFile, *c, c.PrettyPrint)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

func (c *CompressedStackConfig) RenderStackTemplateAsString() (string, error) {
	bytes, err := c.RenderStackTemplateAsBytes()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
