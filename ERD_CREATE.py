import psycopg2
from sqlalchemy import create_engine, MetaData
from sqlalchemy_schemadisplay import create_schema_graph

# Database credentials and details
db_user = "postgres"
db_password = "1QaZ@sXWdE#42"
db_host = "localhost"
db_port = "5555"
db_name = "moneydd-sit"  # Replace with your actual database name

# Create a SQLAlchemy engine
engine_url = (
    f"postgresql+psycopg2://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}"
)
engine = create_engine(engine_url)

# Connect to the PostgreSQL database and reflect the database schema
try:
    # Initialize a MetaData object
    metadata = MetaData()

    # Reflect the schema using a connection
    with engine.connect() as connection:
        metadata.reflect(bind=connection)

    # Generate the ER diagram
    graph = create_schema_graph(
        metadata=metadata,
        show_datatypes=False,  # The diagram will be neater with datatypes off
        show_indexes=False,  # The diagram will be neater with indexes off
        rankdir="LR",  # Left to right layout
        concentrate=False,  # Don't try to join the relation lines together
    )
    graph.write_pdf("erd.pdf")  # Save the diagram as a .pdf file
    print("ERD successfully generated as 'erd.pdf'.")

except psycopg2.Error as e:
    print("Error while connecting to PostgreSQL", e)
except Exception as e:
    print("An error occurred:", e)
finally:
    engine.dispose()  # Properly close the connection
