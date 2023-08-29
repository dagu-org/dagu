package response

func toErrorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
