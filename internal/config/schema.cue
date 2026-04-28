model: {
	url:  string | *"http://localhost:8080/v1"
	name: string | *"local-model"
}
agent: {
	maxRetries:    (int & >=0) | *3
	maxIterations: (int & >=0) | *0
	systemPrompt?: string
}
tools: {
	shell:     {requireConfirmation: bool | *true}
	writeFile: {requireConfirmation: bool | *true}
}
