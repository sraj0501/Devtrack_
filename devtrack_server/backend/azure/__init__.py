"""Azure DevOps integration package."""

# Extend the azure namespace so this package doesn't shadow azure-identity
from pkgutil import extend_path
__path__ = extend_path(__path__, __name__)

from backend.azure.client import AzureDevOpsClient, AzureWorkItem
from backend.azure.sync import AzureSync

__all__ = ["AzureDevOpsClient", "AzureWorkItem", "AzureSync"]
