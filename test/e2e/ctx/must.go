package ctx

func MustProvision(ctx *TestContext) func() {
	deprovision, err := Provision(ctx)
	if err != nil {
		panic(err)
	}
	return deprovision
}
