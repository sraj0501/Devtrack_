"""
SQLAlchemy Database Models for Projects

This module defines the database schema for projects, including relationships
to other entities like backlog stories, tasks, and sprints (created in later phases).
"""

from datetime import datetime
from typing import Optional

try:
    from sqlalchemy import (
        Column, String, Text, DateTime, Float, Enum, Integer,
        ForeignKey, Table, Index, create_engine
    )
    from sqlalchemy.orm import declarative_base, relationship
    SQLALCHEMY_AVAILABLE = True
except ImportError:
    SQLALCHEMY_AVAILABLE = False
    # Stub classes for when SQLAlchemy is not available
    def declarative_base():
        return type('Base', (), {})

    def Column(*args, **kwargs):
        return None


Base = declarative_base() if SQLALCHEMY_AVAILABLE else None


if SQLALCHEMY_AVAILABLE:
    class ProjectDB(Base):
        """SQLAlchemy model for projects table"""
        __tablename__ = 'projects'

        # Primary key and identifiers
        id = Column(String(36), primary_key=True, index=True)
        name = Column(String(255), nullable=False, index=True)
        description = Column(Text)

        # Status and timeline
        status = Column(
            Enum('setup', 'active', 'closed', name='project_status'),
            nullable=False,
            default='setup',
            index=True
        )
        start_date = Column(DateTime(timezone=True), nullable=True)
        end_date = Column(DateTime(timezone=True), nullable=True)

        # Project context
        template_type = Column(
            Enum(
                'web_app', 'library', 'service', 'microservice',
                'cli_tool', 'mobile_app', 'data_pipeline', 'generic',
                name='project_template'
            ),
            default='generic'
        )

        # Resources and risk
        budget_estimate = Column(Float, nullable=True)
        risk_level = Column(
            Enum('low', 'medium', 'high', name='risk_level'),
            default='medium'
        )
        risk_description = Column(Text, default='')

        # Metadata
        created_at = Column(DateTime(timezone=True), default=datetime.utcnow, index=True)
        updated_at = Column(DateTime(timezone=True), default=datetime.utcnow, onupdate=datetime.utcnow)

        # JSON metadata stored as text (serialized)
        metadata_json = Column(Text, default='{}')

        # Relations (will be populated in later phases)
        # backlog_stories = relationship("BacklogStoryDB", back_populates="project")
        # tasks = relationship("TaskDB", back_populates="project")
        # sprints = relationship("SprintDB", back_populates="project")

        def __repr__(self):
            return f"<ProjectDB(id={self.id}, name={self.name}, status={self.status})>"


    # Indexes for common queries
    if Base:
        Index('idx_projects_status', ProjectDB.status)
        Index('idx_projects_created_at', ProjectDB.created_at)
        Index('idx_projects_name', ProjectDB.name)
        Index('idx_projects_status_created', ProjectDB.status, ProjectDB.created_at)


else:
    # Provide a stub class when SQLAlchemy is not available
    class ProjectDB:
        """Stub for when SQLAlchemy is not available"""
        def __init__(self, **kwargs):
            for key, value in kwargs.items():
                setattr(self, key, value)


# Query helper functions for common operations
def query_projects_by_status(session, status: str):
    """Query all projects with a given status."""
    if not SQLALCHEMY_AVAILABLE:
        return []
    return session.query(ProjectDB).filter(ProjectDB.status == status).all()


def query_active_projects(session):
    """Query all currently active projects."""
    if not SQLALCHEMY_AVAILABLE:
        return []
    return session.query(ProjectDB).filter(ProjectDB.status == 'active').all()


def query_recent_projects(session, limit: int = 10):
    """Query recent projects ordered by creation date."""
    if not SQLALCHEMY_AVAILABLE:
        return []
    return session.query(ProjectDB).order_by(ProjectDB.created_at.desc()).limit(limit).all()


def query_project_by_name(session, name: str):
    """Query projects by name (case-insensitive)."""
    if not SQLALCHEMY_AVAILABLE:
        return None
    return session.query(ProjectDB).filter(
        ProjectDB.name.ilike(f"%{name}%")
    ).first()


def query_projects_by_template(session, template_type: str):
    """Query projects by template type."""
    if not SQLALCHEMY_AVAILABLE:
        return []
    return session.query(ProjectDB).filter(
        ProjectDB.template_type == template_type
    ).all()


def query_high_risk_projects(session):
    """Query all high-risk projects."""
    if not SQLALCHEMY_AVAILABLE:
        return []
    return session.query(ProjectDB).filter(ProjectDB.risk_level == 'high').all()


def query_projects_ending_soon(session, days: int = 30):
    """
    Query projects ending within the specified number of days.

    Args:
        session: SQLAlchemy session
        days: Number of days from now to check

    Returns:
        List of ProjectDB objects ending soon
    """
    if not SQLALCHEMY_AVAILABLE:
        return []

    from sqlalchemy import and_
    from datetime import datetime, timedelta

    now = datetime.utcnow()
    future = now + timedelta(days=days)

    return session.query(ProjectDB).filter(
        and_(
            ProjectDB.end_date >= now,
            ProjectDB.end_date <= future,
            ProjectDB.status == 'active'
        )
    ).order_by(ProjectDB.end_date.asc()).all()
