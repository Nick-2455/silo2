package identity

type Identity struct {
	Name      string
	Role      string
	Areas     []string
	Skills    []string
	Interests []string
	Projects  []Project
	Goals     []string
	Evidence  []Evidence
	Outputs   Outputs
}

type Project struct {
	Name        string
	Description string
	Status      string
}

type Evidence struct {
	Source  string
	Summary string
}

type Outputs struct {
	IdentityProfile bool
	CV              bool
	LinkedIn        bool
	Portfolio       bool
	ProfessionalBio bool
}

func DefaultOutputs() Outputs {
	return Outputs{
		IdentityProfile: true,
		CV:              true,
		LinkedIn:        true,
		Portfolio:       true,
		ProfessionalBio: true,
	}
}
