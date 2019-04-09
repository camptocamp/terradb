package importers

// S3Importer implements an Importer interface
type S3Importer struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Endpoint        string
	Location        string
}

// NewS3Importer sets up a S3 importer
func NewS3Importer() (*S3Importer, error) {
	return nil, nil
}
