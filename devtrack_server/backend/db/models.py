from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, EmailStr


class Employee(BaseModel):
	name: str
	emailId: EmailStr
	employeeId: Optional[str] = None
	githubHandle: Optional[str] = None

class AzureTasks(BaseModel):
	id: str
	workItemType: str
	title: str
	assigned_to: Employee
	state: str
	tags: str
	workItemId: str
	devops_url: str
	iteration_path: str
	parentWorkItemId: str
	estimatedTime: int
	remainingTime: int

class DailyTasks(BaseModel):
	taskNo: int
	date: datetime
	mainTask: str
	subTask: str
	activityName: str
	status: str
	priority: str
	comments: str
	startDate: datetime
	endDate: datetime
	assignedTo: Employee
	parentWorkItemId: str
	workItemId: str
	sprint: str

class GitHubActivity(BaseModel):
	repoName: str

class AIModel(Enum):
	OLLAMA = "ollama"
	CLAUDE = "claude"
	OPENAI = "openai"

