module gitlab.com/devtrack3_cloud/devtrack_cli

go 1.24.4

require (
	github.com/joho/godotenv v1.5.1
	gitlab.com/devtrack3_cloud/devtrack_contract v0.0.0
)

replace gitlab.com/devtrack3_cloud/devtrack_contract => ../contract
