package ecsclient

// FailedDeregistration is a struct that couples a task definition deregistration
// error to the ARN of the task definition.
type FailedDeregistration struct {
	Arn string
	Err error
}

// Given two lists of strings, ensures that list B contains none of the items in
// list A. If an A-item is in list B, it is removed from list B. If an A-item is
// not in list B, it is not added to list B. The remaining list B items are returned.
func removeAFromB(a, b []string) []string {
	var diff []string
	m := make(map[string]int)

	for _, item := range b {
		m[item] = 1
	}

	for _, item := range a {
		if m[item] != 0 {
			m[item]++
		}
	}

	for k, v := range m {
		if v == 1 {
			diff = append(diff, k)
		}
	}

	return diff
}
