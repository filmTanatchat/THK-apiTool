import os
import pandas as pd
import json
import requests
import datetime
import time
import re


def remove_comments(json_str):
    """Remove comments from a JSON string."""
    pattern = r"//.*?$|/\*.*?\*/|'(?:\\.|[^'\\])*'|\"(?:\\.|[^\"\\])*\""
    return re.sub(
        pattern,
        lambda m: m.group(0) if m.group(0).startswith(('"', "'")) else "",
        json_str,
        flags=re.MULTILINE | re.DOTALL,
    )


def load_json_from_path(path):
    with open(path, "r", encoding="utf-8") as file:
        json_str = file.read()
        json_str_no_comments = remove_comments(json_str)
        return json.loads(json_str_no_comments)


def authenticate(session, login_url, email, password):
    try:
        auth_payload = {"email": email, "password": password}
        response = session.post(
            login_url, json=auth_payload, headers={"Content-Type": "application/json"}
        )
        response.raise_for_status()
        return {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {response.json()['data']['session_id']}",
        }
    except requests.RequestException as e:
        print(f"Error during authentication: {e}")
        raise


def append_to_log_file(log_file_path, message):
    with open(log_file_path, "a", encoding="utf-8") as log_file:
        log_file.write(message + "\n")


def make_request(session, api_url, headers, payload, log_file_path, case_id):
    try:
        start_time = time.time()
        response = session.post(api_url, json=payload, headers=headers)
        response.raise_for_status()
        end_time = time.time()

        duration = end_time - start_time
        log_message = (
            f"Time: {datetime.datetime.now()}, "
            f"Case ID: {case_id}, "
            f"Response: {response.json()}, "
            f"Execution Time: {duration:.2f} seconds\n"
        )
        append_to_log_file(log_file_path, log_message)
        return {"Response": response.json(), "Status": response.status_code}
    except requests.RequestException as e:
        error_message = (
            f"Time: {datetime.datetime.now()}, "
            f"Case ID: {case_id}, "
            f"Error during request: {e}\n"
        )
        append_to_log_file(log_file_path, error_message)
        return None


def process_column(value):
    # Treat all values as strings, including 'true', 'false', or numeric strings like '000003'
    return value if pd.notna(value) else ""


def create_json_payload(row, columns):
    answers = [
        {
            "index": idx,
            "field_name": col,
            "field_value": process_column(
                row[col]
            ),  # Process each column value as string
            "source": "customer",
        }
        for idx, col in enumerate(columns, start=1)
        if col != "case_id"
    ]
    return {
        "case_id": str(row["case_id"]),
        "is_question_mode": False,
        "answers": answers,
    }


def main():
    base_path = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    env_path = os.path.join(base_path, "1. environment", "env.json")
    log_file_path = os.path.join(base_path, "2. log", "answerMultiQuestion.log")

    if not os.path.exists(env_path):
        print(f"env.json not found at {env_path}")
        return

    env = load_json_from_path(env_path)

    with requests.Session() as session:
        headers = authenticate(
            session,
            f"{env['BASE_URL']}/authentication/api/v1/login",
            env["EMAIL"],
            env["PASSWORD"],
        )

        csv_path = os.path.join(
            base_path, "4. answerAndQuestion", "answerAllFields.csv"
        )

        try:
            df = pd.read_csv(csv_path, dtype=str, encoding="utf-8")
            print("Sample data from CSV:", df.head())
        except FileNotFoundError:
            print(f"CSV file not found: {csv_path}")
            return

        for _, row in df.iterrows():
            if pd.isna(row["case_id"]) or row["case_id"] == "":
                continue

            payload = create_json_payload(row, df.columns)
            case_id = row["case_id"]
            result = make_request(
                session,
                f"{env['BASE_URL']}/question-taskpool/api/v1/answer-question",
                headers,
                payload,
                log_file_path,
                case_id,
            )

            if result:
                # Print only case_id and response status
                print(f"Case ID: {case_id}, Status: {result['Status']}")

        # Save your processed data to JSON as well if needed
        # Example: Saving DataFrame to JSON
        json_output_path = os.path.join(base_path, "processed_data.json")
        with open(json_output_path, "w", encoding="utf-8") as f:
            df.to_json(f, force_ascii=False, indent=4)


if __name__ == "__main__":
    main()
