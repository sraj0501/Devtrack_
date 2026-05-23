"""
SQLAlchemy database models for DevTrack.

This package contains the database models for the core entities in the system.
"""

try:
    from backend.db.models.project import (
        ProjectDB,
        query_projects_by_status,
        query_active_projects,
        query_recent_projects,
        query_project_by_name,
        query_projects_by_template,
        query_high_risk_projects,
        query_projects_ending_soon,
    )

    __all__ = [
        'ProjectDB',
        'query_projects_by_status',
        'query_active_projects',
        'query_recent_projects',
        'query_project_by_name',
        'query_projects_by_template',
        'query_high_risk_projects',
        'query_projects_ending_soon',
    ]
except ImportError:
    # SQLAlchemy not available
    __all__ = []
