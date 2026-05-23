from setuptools import setup, find_packages

setup(
    name="git-sage",
    version="0.1.0",
    description="Local LLM-powered git assistant",
    packages=find_packages(),
    python_requires=">=3.10",
    entry_points={
        "console_scripts": [
            "git-sage=git_sage.cli:main",
        ],
    },
)
