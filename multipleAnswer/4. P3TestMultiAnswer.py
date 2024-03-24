import asyncio
import aiohttp
import os
import json
import csv
import re
import time

BASE_PATH = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
LOG_DIR = os.path.join(BASE_PATH, "2. log")
LOG_FILE_PATH = os.path.join(LOG_DIR, "responses_P3.log")


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
    with open(os.path.join(BASE_PATH, path), "r") as file:
        json_str = file.read()
        json_str_no_comments = remove_comments(json_str)
        return json.loads(json_str_no_comments)


def read_csv_and_prepare_payloads(csv_path, json_payload_template):
    with open(csv_path, newline="", encoding="utf-8") as csvfile:
        reader = csv.DictReader(csvfile)
        payloads = []
        for row in reader:
            payload = json_payload_template.copy()
            for key, value in payload.items():
                if isinstance(value, str):
                    for column_name in row:
                        if "{{" + column_name + "}}" in value:
                            payload[key] = value.replace(
                                "{{" + column_name + "}}", row[column_name]
                            )
            payloads.append(payload)
        return payloads


async def authenticate_async(session, login_url, email, password):
    try:
        auth_payload = {"email": email, "password": password}
        async with session.post(
            login_url, json=auth_payload, headers={"Content-Type": "application/json"}
        ) as response:
            response.raise_for_status()
            data = await response.json()
            return f"Bearer {data['data']['session_id']}"
    except Exception as e:
        print(f"Error during authentication: {e}")
        return None


async def async_make_and_process_request(session, api_url, headers, payload):
    try:
        start_time = time.perf_counter()
        async with session.post(api_url, json=payload, headers=headers) as response:
            response.raise_for_status()
            response_data = await response.json()
            end_time = time.perf_counter()
            return response_data, response.status, end_time - start_time
    except Exception as e:
        return {"error": str(e)}, 500, None


async def main_async():
    if not os.path.exists(LOG_DIR):
        os.makedirs(LOG_DIR)

    ENV = load_json_from_path("1. environment/env.json")
    SUBMIT_PAYLOAD_TEMPLATE = load_json_from_path("3. dataSource/P3.json")
    CSV_PATH = os.path.join(BASE_PATH, "3. datasource", "multiCaseId.csv")

    modified_payloads = read_csv_and_prepare_payloads(CSV_PATH, SUBMIT_PAYLOAD_TEMPLATE)

    async with aiohttp.ClientSession() as session:
        # Authenticate and retrieve token
        token = await authenticate_async(
            session,
            f"{ENV['BASE_URL']}/authentication/api/v1/login",
            ENV["EMAIL"],
            ENV["PASSWORD"],
        )
        if not token:
            print("Authentication failed. Exiting.")
            return

        headers = {"Content-Type": "application/json", "Authorization": token}

        tasks = [
            asyncio.ensure_future(
                async_make_and_process_request(
                    session,
                    f"{ENV['BASE_URL']}/question-taskpool/api/v1/answer-question",
                    headers,
                    payload,
                )
            )
            for payload in modified_payloads
        ]

        responses = await asyncio.gather(*tasks)

        with open(LOG_FILE_PATH, "w", encoding="utf-8") as log_file:
            for response_data, status_code, response_time in responses:
                log_entry = {
                    "Status Code": status_code,
                    "Response Time (s)": response_time,
                    "Response Body": response_data,
                }
                log_file.write(
                    json.dumps(log_entry, indent=4, ensure_ascii=False) + "\n\n"
                )


if __name__ == "__main__":
    asyncio.run(main_async())
