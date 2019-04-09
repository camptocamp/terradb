package importers

// Importer implements a Terraform state files importer interface
type Importer interface {
	GetName() string
}
