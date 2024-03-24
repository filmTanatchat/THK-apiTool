import csv
import requests
import os
import json
import re

# Constants
MAX_RETRIES_PER_ROW = 3

# Directory paths
main_folder_path = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))


def remove_comments(json_str):
    """Remove comments from a JSON string."""
    pattern = r'//.*?$|/\*.*?\*/|\'(?:\\.|[^\'\\])*\'|"(?:\\.|[^"\\])*"'
    return re.sub(
        pattern,
        lambda m: m.group(0) if m.group(0).startswith(('"', "'")) else "",
        json_str,
        flags=re.MULTILINE | re.DOTALL,
    )


ENV_PATH = os.path.join(main_folder_path, "1. environment", "env.json")
LOG_FILE = os.path.join(main_folder_path, "2. log", "formUpdateFieldLog.json")
CSV_FILE = os.path.join(
    main_folder_path, "3. dataSource", "additionalFormAddUpdateField.csv"
)

# Load environment variables
with open(ENV_PATH, "r") as file:
    json_str = file.read()
    cleaned_json_str = remove_comments(json_str)
    ENV = json.loads(cleaned_json_str)

BASE_URL = ENV["BASE_URL"]
LOGIN_URL = f"{BASE_URL}/authentication/api/v1/login"
API_URL = f"{BASE_URL}/form/api/v1/update_field"


def post_request_with_retries(session, headers, row):
    for _ in range(MAX_RETRIES_PER_ROW):
        payload = {
            "form_name": row["form_name"],
            "field_name": row["field_name"],
            "is_mandatory": row["is_mandatory"].lower() == "true",
        }
        response = session.post(API_URL, json=payload, headers=headers)
        if response.status_code == 200:
            return response
    return response


def process_file(session, headers):
    logs = []
    response_counters = {}

    with open(CSV_FILE, mode="r", encoding="utf-8-sig") as file:
        reader = csv.DictReader(file)
        for row in reader:
            response = post_request_with_retries(session, headers, row)
            status = response.status_code
            logs.append(
                {
                    "Request_URL": response.url,
                    "Response": response.json(),
                    "Status": status,
                }
            )
            response_counters[status] = response_counters.get(status, 0) + 1

    return logs, response_counters


def main():
    auth_payload = {"email": ENV["EMAIL"], "password": ENV["PASSWORD"]}

    with requests.Session() as session:
        response = session.post(
            LOGIN_URL, json=auth_payload, headers={"Content-Type": "application/json"}
        )
        response.raise_for_status()

        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {response.json()['data']['session_id']}",
        }

        logs, response_counters = process_file(session, headers)

        # Save logs
        with open(LOG_FILE, "w", encoding="utf-8") as log_file:
            json.dump(logs, log_file, ensure_ascii=False, indent=4)

        # Print summarized results
        for status, count in response_counters.items():
            print(f"URL: {ENV['BASE_URL']}")
            print(f"status {status} : {count} Fields.")


if __name__ == "__main__":
    main()
