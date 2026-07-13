import os

from superset import db
from superset.app import create_app
from superset.models.core import Database


database_name = os.environ.get(
    "SUPERSET_OPERATION_DWH_DATABASE_NAME",
    "Mock Data Warehouse",
)
database_uri = os.environ.get(
    "SUPERSET_OPERATION_DWH_SQLALCHEMY_URI",
    "postgresql+psycopg2://bi_user:bi_password@mock-data-warehouse:5432/bi_warehouse",
)

app = create_app()
with app.app_context():
    database = db.session.query(Database).filter_by(database_name=database_name).one_or_none()
    if database is None:
        database = Database(database_name=database_name)
        db.session.add(database)

    database.set_sqlalchemy_uri(database_uri)
    database.expose_in_sqllab = True
    database.allow_run_async = False
    db.session.commit()

print(f"Provisioned Superset database connection: {database_name}", flush=True)
