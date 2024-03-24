import requests
import os
import json
import re
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
    ENV = load_json_from_path(os.path.join(BASE_PATH, "1. environment", "env.json"))
    PAYLOAD = load_json_from_path(
        os.path.join(BASE_PATH, "4. answerAndQuestion", "answerAllFields.json")
    )

    with requests.Session() as session:
        headers = authenticate(
            session,
            f"{ENV['BASE_URL']}/authentication/api/v1/login",
            ENV["EMAIL"],
            ENV["PASSWORD"],
        )

        logs = []
        response_counters = defaultdict(int)
        unique_responses = defaultdict(lambda: defaultdict(int))

        success = False
        attempts = 0
        while not success and attempts < 10:
            result = make_request(
                session,
                f"{ENV['BASE_URL']}/question-taskpool/api/v1/answer-question",
                headers,
                PAYLOAD,
            )
            logs.append(result)
            response_counters[result["Status"]] += 1
            response_str = json.dumps(result["Response"])
            unique_responses[result["Status"]][response_str] += 1
            if result["Status"] == 200:
                success = True
            attempts += 1

        # Optimize logs for saving
        unique_responses_set = set(json.dumps(log["Response"]) for log in logs)
        logs_to_save = [
            logs[0] if len(unique_responses_set) == 1 else log for log in logs
        ]

        # Save the logs to the file
        with open(
            os.path.join(BASE_PATH, "2. log", "answerQuestionLog.json"),
            "w",
            encoding="utf-8",
        ) as log_file:
            json.dump(logs_to_save, log_file, ensure_ascii=False, indent=4)

        # Print results
        for status, count in response_counters.items():
            print(f"status {status} : {count} Requests.")
            print(f"detail of {status}:")
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
