#!/usr/bin/env python3
"""Quick import test to verify syntax and basic imports work"""

import sys
import traceback

def test_import(module_name):
    """Test importing a module and return success status"""
    try:
        __import__(module_name)
        print(f"✓ {module_name}")
        return True
    except Exception as e:
        print(f"✗ {module_name}")
        print(f"  Error: {str(e)}")
        traceback.print_exc()
        return False

# Test imports
modules = [
    "backend.models.project",
    "backend.models",
    "backend.project_manager",
    "backend.db.models.project",
    "backend.db.models",
    "backend.tests.test_project_manager",
    "backend.tests.test_project_integration",
]

print("Testing imports...\n")
results = [test_import(m) for m in modules]
print(f"\nResults: {sum(results)}/{len(results)} modules imported successfully")

sys.exit(0 if all(results) else 1)
