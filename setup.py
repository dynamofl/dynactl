"""
Setup script for dynactl
"""

from setuptools import setup, find_packages

setup(
    name="dynactl",
    version="0.1.0",
    description="Dynamo AI Deployment Tool",
    author="Dynamo AI",
    author_email="info@dynamoai.com",
    packages=find_packages(),
    install_requires=[
        "click>=8.0.0",
        "pyyaml>=6.0",
        "requests>=2.25.0",
        "kubernetes>=24.2.0",
    ],
    entry_points={
        "console_scripts": [
            "dynactl=dynactl.cli:main",
        ],
    },
    python_requires=">=3.8",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "Intended Audience :: System Administrators",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Topic :: Software Development :: Build Tools",
        "Topic :: System :: Systems Administration",
    ],
) 