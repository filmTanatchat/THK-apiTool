import os
import pandas as pd
import json
import requests
import csv
import datetime
import time
import re

# Function Definitions


def remove_comments(json_str):
    """Remove comments from a JSON string."""
    pattern = r"""
        (?:"(?:\\.|[^"\\])*")  # Double quoted strings
        |(?:'(?:\\.|[^'\\])*')  # Single quoted strings
        |(?:\/\/[^\r\n]*)       # Single-line comments
        |(?:\/\*[\s\S]*?\*\/)   # Multi-line comments
    """
    return re.sub(
        pattern,
        lambda m: "" if m.group(0).startswith(("/", "'")) else m.group(0),
        json_str,
        flags=re.VERBOSE,
    )


def load_json_from_path(path):
    with open(path, "r", encoding="utf-8") as file:
        json_str = file.read()
        json_str_no_comments = remove_comments(json_str)
        return json.loads(json_str_no_comments)


def authenticate(session, login_url, email, password):
    auth_payload = {"email": email, "password": password}
    response = session.post(
        login_url, json=auth_payload, headers={"Content-Type": "application/json"}
    )
    response.raise_for_status()
    return {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {response.json()['data']['session_id']}",
    }


def append_to_log_file(log_file_path, message):
    with open(log_file_path, "a", encoding="utf-8") as log_file:
        log_file.write(message + "\n")


def process_form(session, api_url, headers, log_file_path, form_name, field_names):
    results = []
    for field_name in field_names:
        payload = {
            "form_name": form_name,
            "names": [field_name],
        }
        json_payload = json.dumps(payload)

        try:
            delete_response = session.delete(
                api_url, data=json_payload, headers=headers
            )
            delete_response.raise_for_status()
            response_json = delete_response.json()
            status = delete_response.status_code
            print(f"Field Name: {field_name}, Status: {status}")  # Print to console
        except requests.HTTPError as http_err:
            response_json = {"error": "HTTP error occurred", "details": str(http_err)}
            status = delete_response.status_code if delete_response else "No Response"
            print(
                f"Field Name: {field_name}, Status: {status}, Error: {http_err}"
            )  # Print to console
        except Exception as e:
            response_json = {"error": "Unexpected error", "details": str(e)}
            status = "Error"
            print(
                f"Field Name: {field_name}, Status: {status}, Error: {e}"
            )  # Print to console

        result = {
            "form_name": form_name,
            "field_name": field_name,
            "Request_URL": delete_response.url,
            "Response": response_json,
            "Status": status,
        }
        results.append(result)

        log_message = json.dumps(result, ensure_ascii=False, indent=4)
        append_to_log_file(log_file_path, log_message)

    return results


def aggregate_field_names_from_csv(csv_file):
    field_name_map = {}
    with open(csv_file, mode="r", encoding="utf-8-sig") as file:
        reader = csv.DictReader(file)
        for row in reader:
            form_name = row.get("form_name", "")
            field_name = row.get("field_name", "")
            if form_name not in field_name_map:
                field_name_map[form_name] = []
            field_name_map[form_name].append(field_name)
    return field_name_map


# Main script execution

main_folder_path = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
ENV_PATH = os.path.join(main_folder_path, "1. environment", "env.json")
LOG_FILE = os.path.join(main_folder_path, "2. log", "formDeleteFieldLog.json")
CSV_FILE = os.path.join(
    main_folder_path, "3. dataSource", "additionalFormAddUpdateField.csv"
)

env = load_json_from_path(ENV_PATH)
BASE_URL = env["BASE_URL"]
LOGIN_URL = f"{BASE_URL}/authentication/api/v1/login"
API_URL = f"{BASE_URL}/form/api/v1/delete_field"

with requests.Session() as session:
    headers = authenticate(session, LOGIN_URL, env["EMAIL"], env["PASSWORD"])
    field_name_map = aggregate_field_names_from_csv(CSV_FILE)

    all_results = []
    for form_name, field_names in field_name_map.items():
        results = process_form(
            session, API_URL, headers, LOG_FILE, form_name, field_names
        )
        all_results.extend(results)

    with open(LOG_FILE, "w", encoding="utf-8") as log_file:
        json.dump(all_results, log_file, ensure_ascii=False, indent=4)
