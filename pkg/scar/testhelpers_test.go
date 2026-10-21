package scar

// failToPassVerifier builds a verifier whose command hash is consistent with its
// argv, as required for an active scar that claims fail-to-pass verification.
func failToPassVerifier(argv ...string) Verifier {
	return Verifier{
		Argv:        argv,
		Result:      VerificationFailToPass,
		CommandHash: ComputeCommandHash(argv, "", nil),
	}
}
