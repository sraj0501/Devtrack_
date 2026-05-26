import json
import logging
import os
import sys

import pytz
from dotenv import load_dotenv
from github import Github, Auth, GithubException

# Add project root for config
_script_dir = os.path.dirname(os.path.abspath(__file__))
_project_root = os.path.dirname(os.path.dirname(_script_dir))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)


class GitHubBranchAnalyzer:
    def __init__(self, token=None):
        """
        Initialize GitHub Branch Analyzer with robust authentication and logging.

        This method ensures secure access to GitHub and sets up comprehensive logging.
        """
        # Load environment from project root
        from backend.config import github_token, timezone, log_file_path, get_github_log_path
        self.token = token or github_token()
        tz_name = timezone()
        log_path_val = get_github_log_path()
        log_path = log_path_val if log_path_val else str(log_file_path())

        if not self.token:
            raise ValueError("GitHub Personal Access Token is required (GITHUB_TOKEN in .env)")

        # Configure authentication
        self.auth = Auth.Token(self.token)
        self.github = Github(auth=self.auth)

        # Configure logging
        log_dir = os.path.dirname(log_path)
        if log_dir and not os.path.exists(log_dir):
            os.makedirs(log_dir, exist_ok=True)
        logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(levelname)s: %(message)s',
            handlers=[
                logging.FileHandler(log_path),
                logging.StreamHandler(sys.stdout)
            ]
        )
        self.logger = logging.getLogger(__name__)

        # Configure timezone
        self.ist_tz = pytz.timezone(tz_name)


    def getRepos(self):
        """
        Retrieve repositories and their branches for the authenticated user.

        Returns:
        dict: A dictionary with repository names as keys and lists of branch names as values.
        """
        # auth = Auth.Token(token)
        g = Github(auth=self.auth)
        repo_list = {}

        try:
            for repo in g.get_user().get_repos():
                print("Processing repo:", repo.name)

                # Collect all branches for the current repository
                branches = [branch.name for branch in repo.get_branches()]

                # Store repository name and its branches
                repo_list[repo.name] = branches

        except Exception as e:
            print(f"Error retrieving repositories: {e}")

        finally:
            g.close()

        return repo_list


    def analyze_repository_branches(self, repository_name):
        """
        Analyze branches in a specific repository, extracting detailed branch information.

        Args:
            repository_name (str): Full name of the repository (e.g., 'username/repo')

        Returns:
            list: Detailed information about each branch
        """
        try:
            # Retrieve the repository
            repo = self.github.get_repo(repository_name)

            branch_details = []

            # Iterate through all branches
            for branch in repo.get_branches():
                try:
                    # Get the latest commit for this branch
                    commit = repo.get_commit(branch.commit.sha)

                    # Convert commit time to IST
                    commit_time_utc = commit.commit.committer.date
                    commit_time_ist = commit_time_utc.astimezone(self.ist_tz)

                    # Calculate total lines modified
                    lines_modified = commit.stats.total if commit.stats else 0

                    branch_info = {
                        'repository_name': repository_name,
                        'branch_name': branch.name,
                        'last_modified_time_ist': commit_time_ist.strftime('%Y-%m-%d %H:%M:%S %Z'),
                        'lines_modified': lines_modified,
                        'author_name': commit.commit.author.name,
                        'author_email': commit.commit.author.email
                    }

                    branch_details.append(branch_info)

                    # Log successful branch processing
                    self.logger.info(f"Processed branch: {branch.name}")

                except Exception as branch_error:
                    self.logger.error(f"Error processing branch {branch.name}: {branch_error}")

            return branch_details

        except Exception as repo_error:
            self.logger.error(f"Error analyzing repository {repository_name}: {repo_error}")
            return []


    def export_branch_analysis(self, branch_details, output_file='branch_analysis.json'):
        """
        Export branch analysis results to a JSON file.

        Args:
            branch_details (list): List of branch detail dictionaries
            output_file (str): Filename for JSON export
        """
        try:
            with open(output_file, 'w', encoding='utf-8') as f:
                json.dump(branch_details, f, indent=2, ensure_ascii=False)

            self.logger.info(f"Exported branch analysis for {len(branch_details)} branches to {output_file}")

        except Exception as e:
            self.logger.error(f"Failed to export branch analysis: {e}")
            raise e


    def analyze_repository_commits(self, repository_name):
        """
		Analyze all commits in all branches of a specific repository,
		extracting detailed commit information.

		Args:
			repository_name (str): Full name of the repository (e.g., 'username/repo')

		Returns:
			list: Detailed information about each commit in all branches
		"""
        try:
            # Retrieve the repository
            repo = self.github.get_repo(repository_name)

            commit_details = []

            # Iterate through all branches
            for branch in repo.get_branches():
                try:
                    self.logger.info(f"Analyzing branch: {branch.name}")

                    # Iterate through all commits in the branch
                    commits = repo.get_commits(sha=branch.name)
                    for commit in commits:
                        try:
                            # Convert commit time to IST
                            commit_time_utc = commit.commit.committer.date
                            commit_time_ist = commit_time_utc.astimezone(self.ist_tz)

                            # Calculate total lines modified
                            lines_modified = commit.stats.total if commit.stats else 0

                            # Commit details
                            commit_info = {
                                'repository_name': repository_name,
                                'branch_name': branch.name,
                                'commit_sha': commit.sha,
                                'commit_message': commit.commit.message,
                                'last_modified_time_ist': commit_time_ist.strftime('%Y-%m-%d %H:%M:%S %Z'),
                                'lines_modified': lines_modified,
                                'author_name': commit.commit.author.name,
                                'author_email': commit.commit.author.email
                            }

                            commit_details.append(commit_info)

                            # Log successful commit processing
                            self.logger.info(f"Processed commit: {commit.sha} on branch {branch.name}")

                        except Exception as commit_error:
                            self.logger.error(f"Error processing commit {commit.sha} on branch {branch.name}: {commit_error}")

                except Exception as branch_error:
                    self.logger.error(f"Error processing branch {branch.name}: {branch_error}")

            return commit_details

        except Exception as repo_error:
            self.logger.error(f"Error analyzing repository {repository_name}: {repo_error}")
            return []


def main():
    try:
        branch_analyzer = GitHubBranchAnalyzer()
        repolist = branch_analyzer.getRepos()
        for repo, branches in repolist.items():
            print(repo)
            print(branches)
            repository_name = f'{os.getenv("USER_NAME")}/{repo}'
            commit_analysis = branch_analyzer.analyze_repository_commits(repository_name)
            branch_analyzer.export_branch_analysis(commit_analysis, output_file=f'{repo}.json')
            print("*"*10)
    except Exception as e:
        print(f"Analysis failed: {e}")


if __name__ == "__main__":
    main()