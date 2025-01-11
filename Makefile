all:
	go run .

debug:
	go run . -log debug

warn:
	go run . -log warn
