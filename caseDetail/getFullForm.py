import requests
import os
import json
import re

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


def authenticate(session, env):
    auth_payload = {"email": env["EMAIL"], "password": env["PASSWORD"]}
    response = session.post(
        f"{env['BASE_URL']}/authentication/api/v1/login",
        json=auth_payload,
        headers={"Content-Type": "application/json"},
    )
    response.raise_for_status()
    return {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {response.json()['data']['session_id']}",
    }


def make_request(session, api_url, headers, payload):
    response = session.post(api_url, json=payload, headers=headers)
    return {"Response": response.json(), "Status": response.status_code}


def log_response(response, path):
    with open(path, "w", encoding="utf-8") as log_file:
        json.dump(response, log_file, ensure_ascii=False, indent=4)


def print_status_summary(response_counters, unique_responses):
    for status, count in response_counters.items():
        print(f"status {status} : {count} Requests.")
        print(f"detail of {status}:")
        if status == 200:
            response_str, occurrence = list(unique_responses[status].items())[0]
            resp = json.loads(response_str)
            print(f"Occurrences: {count}")
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


def main():
    ENV = load_json_from_path("1. environment/env.json")
    PAYLOAD = load_json_from_path("3. dataSource/productName.json")

    with requests.Session() as session:
        headers = authenticate(session, ENV)
        response_apply_product = make_request(
            session,
            f"{ENV['BASE_URL']}/question-taskpool/api/v1/apply-for-product",
            headers,
            PAYLOAD,
        )

        if (
            response_apply_product["Status"] == 200
            and "data" in response_apply_product["Response"]
            and "case_id" in response_apply_product["Response"]["data"]
        ):
            case_id = response_apply_product["Response"]["data"]["case_id"]

            PAYLOAD_GET_FULL_FORM = {"case_id": case_id}
            response_get_full_form = make_request(
                session,
                f"{ENV['BASE_URL']}/question-taskpool/api/v1/get-full-form",
                headers,
                PAYLOAD_GET_FULL_FORM,
            )

            log_response(
                response_get_full_form,
                os.path.join(BASE_PATH, "2. log", "getFullFormLog.json"),
            )

            response_counters = {response_get_full_form["Status"]: 1}
            unique_responses = {
                response_get_full_form["Status"]: {
                    json.dumps(response_get_full_form["Response"]): 1
                }
            }

            print_status_summary(response_counters, unique_responses)
        else:
            print("Failed to apply for product or extract case_id")
            print(f"Status Code: {response_apply_product['Status']}")
            print(
                f"Response: {json.dumps(response_apply_product['Response'], indent=2)}"
            )


if __name__ == "__main__":
    main()
