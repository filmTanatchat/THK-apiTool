import csv
import requests
import os
import json
import re
from concurrent.futures import ThreadPoolExecutor, as_completed
from collections import defaultdict

main_folder_path = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))


BASE_PATH = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))


def remove_comments(json_str):
    """Remove comments from a JSON string."""
    pattern = r"//.*?$|/\*.*?\*/|'(?:(?:\\.|[^'\\])*)'|\"(?:(?:\\.|[^\"\\])*)\""
    return re.sub(
        pattern,
        lambda m: m.group(0) if m.group(0).startswith(('"', "'")) else "",
        json_str,
        flags=re.MULTILINE | re.DOTALL,
    )


def load_json_from_path(path):
    with open(os.path.join(BASE_PATH, path), "r") as file:
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


def process_row(session, api_url, headers, row, row_number):
    payload = {
        "name": row.get("name", ""),
        "label": {
            "en": {
                "text": row.get("label_en_text", ""),
                "image_url": row.get("label_en_image_url", ""),
            },
            "th": {
                "text": row.get("label_th_text", ""),
                "image_url": row.get("label_th_image_url", ""),
            },
        },
        "data_type": row.get("data_type", "").lower(),
        "source_of_value": row.get("source_of_value", ""),
        "is_manual_allowed": row.get("is_manual_allowed", "").lower() == "true",
        "filled_when_apply_product": row.get("filled_when_apply_product", "").lower()
        == "true",
        "default_value": row.get("default_value", ""),
        "default_condition": row.get("default_condition", ""),
        "formula": row.get("formula", ""),
        "is_active": row.get("is_active", "").lower() == "true",
        "is_multiple_values_allowed": row.get("is_multiple_values_allowed", "").lower()
        == "true",
        "parent_hierarchy_field_names": [
            item
            for item in row.get("parent_hierarchy_field_names", "").split(",")
            if item
        ],
        "triggering_module_names": [
            item for item in row.get("triggering_module_names", "").split(",") if item
        ],
        "validation_name": row.get("validation_name", ""),
        "choice_name": row.get("choice_name", ""),
    }

    try:
        post_response = session.post(api_url, json=payload, headers=headers)
        return {
            "Name": row.get("name"),
            "FromRow": row_number,
            "Response": post_response.json(),
            "Status": post_response.status_code,
        }
    except requests.RequestException as e:
        return {
            "Name": row.get("name"),
            "FromRow": row_number,
            "Error": str(e),
        }


def main():
    ENV = load_json_from_path(os.path.join(BASE_PATH, "1. environment", "env.json"))
    BASE_URL = ENV["BASE_URL"]

    with requests.Session() as session:
        headers = authenticate(
            session,
            f"{BASE_URL}/authentication/api/v1/login",
            ENV["EMAIL"],
            ENV["PASSWORD"],
        )

        logs = []
        response_counters = defaultdict(int)
        unique_responses = defaultdict(lambda: defaultdict(int))

        with open(
            os.path.join(main_folder_path, "3. dataSource", "fieldCreateAndUpdate.csv"),
            mode="r",
            encoding="utf-8-sig",
        ) as file:
            reader = csv.DictReader(file)
            with ThreadPoolExecutor() as executor:
                futures = [
                    executor.submit(
                        process_row,
                        session,
                        f"{BASE_URL}/universe/api/v1/update_field",
                        headers,
                        row,
                        row_number,
                    )
                    for row_number, row in enumerate(reader, 1)
                ]
                for future in as_completed(futures):
                    result = future.result()
                    logs.append(result)
                    response_counters[result["Status"]] += 1
                    response_str = json.dumps(result["Response"])
                    unique_responses[result["Status"]][response_str] += 1

        with open(
            os.path.join(main_folder_path, "2. log", "fieldCreateLog.json"),
            "w",
            encoding="utf-8",
        ) as log_file:
            json.dump(logs, log_file, ensure_ascii=False, indent=4)

        # Print summarized results to the terminal
        for status, count in response_counters.items():
            print(f"URL: {ENV['BASE_URL']}")
            print(f"status {status} : {count} Fields.")
            print(f"detail of {status}:")

            for response_str, occurrence in unique_responses[status].items():
                resp = json.loads(response_str)
                print(f"Occurrences: {occurrence}")
                print(f"Code: {resp.get('code')}, Message: {resp.get('message')}")
                for error in resp.get("errors", []):
                    print(
                        f"Error Code: {error.get('code')}, Field: {error.get('field_name')}, Message: {error.get('message')}"
                    )
                print("----------------------")  # Separator for clarity


if __name__ == "__main__":
    main()
