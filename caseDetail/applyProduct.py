import requests
import os
import json
import re
from concurrent.futures import ThreadPoolExecutor, as_completed
from collections import defaultdict

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


def make_request(session, api_url, headers, payload):
    response = session.post(api_url, json=payload, headers=headers)
    return {"Response": response.json(), "Status": response.status_code}


def main():
    ENV = load_json_from_path("1. environment/env.json")
    PAYLOAD = load_json_from_path("3. dataSource/productName.json")

    with requests.Session() as session:
        headers = authenticate(
            session,
            f"{ENV['BASE_URL']}/authentication/api/v1/login",
            ENV["EMAIL"],
            ENV["PASSWORD"],
        )

        response_counters = defaultdict(int)
        unique_responses = defaultdict(lambda: defaultdict(int))

        with ThreadPoolExecutor() as executor:
            futures = [
                executor.submit(
                    make_request,
                    session,
                    f"{ENV['BASE_URL']}/question-taskpool/api/v1/apply-for-product",
                    headers,
                    PAYLOAD,
                )
                for _ in range(1)
            ]
            logs = [future.result() for future in as_completed(futures)]
            for log in logs:
                response_str = json.dumps(log["Response"])
                response_counters[log["Status"]] += 1
                unique_responses[log["Status"]][response_str] += 1

        logs_to_save = [
            log if len(unique_responses[log["Status"]]) > 1 else logs[0] for log in logs
        ]

        with open(
            os.path.join(BASE_PATH, "2. log", "applyProductDetail.json"),
            "w",
            encoding="utf-8",
        ) as log_file:
            json.dump(logs_to_save, log_file, ensure_ascii=False, indent=4)

        for status, count in response_counters.items():
            print(f"URL: {ENV['BASE_URL']}")
            print(f"status {status} : {count} Requests.")
            print(f"detail of {status}:")

            if status == 200:
                response_str, occurrence = list(unique_responses[status].items())[0]
                resp = json.loads(response_str)
                print(
                    f"Occurrences: {count}"
                )  # Print total count instead of individual occurrences
                print(f"Code: {resp.get('code')}, Message: {resp.get('message')}")
                print("----------------------")
            else:
                for response_str, occurrence in unique_responses[status].items():
                    resp = json.loads(response_str)
                    print(f"Occurrences: {occurrence}")
                    print(f"Code: {resp.get('code')}, Message: {resp.get('message')}")
                    for error in resp.get("errors", []):
                        print(
                            f"Error Code: {error.get('code')}, Field: {error.get('field_name')}, Message: {error.get('message')}"
                        )
                    print("----------------------")


if __name__ == "__main__":
    main()
