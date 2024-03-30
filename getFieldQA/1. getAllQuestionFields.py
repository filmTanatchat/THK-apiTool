import requests
import os
import json
import csv
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


def extract_csv_data(response_json, field_type="all"):
    data = response_json.get("Response", {}).get("data", {})
    fields = data.get("fields", [])
    additional_fields = data.get("additional_fields", [])

    if field_type == "fields":
        all_fields = fields
    elif field_type == "additional_fields":
        all_fields = additional_fields
    else:  # "all"
        all_fields = fields + additional_fields

    case_id = data.get("case_id", "")

    header = ["case_id"]
    example_data_row = [case_id]

    for field in all_fields:
        field_name = field["field_name"]
        input_source = field.get("input_source", "")

        if input_source == "":
            field_name_for_header = field_name + f"||{field['data_type']}"
            if field["is_multiple_values_allowed"]:
                field_name_for_header += "||MULTI"
            header.append(field_name_for_header)

            example_data = ""
            if field["data_type"] == "date":
                example_data = "DD-MM-YYYY"
            elif field["data_type"] == "date_time":
                example_data = "DD-MM-YYYY hh:mm:ss"
            elif field["data_type"] == "number":
                example_data = "0"
            elif field["data_type"] == "file":
                example_data = "test1.pdf"
            elif field["data_type"] == "text" and field["is_multiple_values_allowed"]:
                example_data = "text1\\text2"
            elif field["data_type"] == "number" and field["is_multiple_values_allowed"]:
                example_data = "0\\0"
            elif field["data_type"] == "file" and field["is_multiple_values_allowed"]:
                example_data = "test1.pdf\\test2.pdf"

            example_data_row.append(example_data)

    return header, example_data_row


def write_csv(header, row, path):
    with open(path, mode="w", newline="", encoding="utf-8") as file:
        writer = csv.writer(file)
        writer.writerow(header)
        writer.writerow(row)


def main():
    ENV = load_json_from_path("1. environment/env.json")
    PAYLOAD = load_json_from_path("3. dataSource/productName.json")

    response_counters = defaultdict(int)
    unique_responses = defaultdict(lambda: defaultdict(int))

    with requests.Session() as session:
        headers = authenticate(
            session,
            f"{ENV['BASE_URL']}/authentication/api/v1/login",
            ENV["EMAIL"],
            ENV["PASSWORD"],
        )

        response_apply_product = make_request(
            session,
            f"{ENV['BASE_URL']}/question-taskpool/api/v1/apply-for-product",
            headers,
            PAYLOAD,
        )

        response_str = json.dumps(response_apply_product["Response"])
        status = response_apply_product["Status"]
        response_counters[status] += 1
        unique_responses[status][response_str] += 1

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

            with open(
                os.path.join(BASE_PATH, "2. log", "getFullFormLog.json"),
                "w",
                encoding="utf-8",
            ) as log_file:
                json.dump(
                    response_get_full_form, log_file, ensure_ascii=False, indent=4
                )

            # Write all fields to CSV
            header_all, row_all = extract_csv_data(response_get_full_form, "all")
            write_csv(
                header_all,
                row_all,
                os.path.join(
                    BASE_PATH, "4. answerAndQuestion", "questionAllFields.csv"
                ),
            )

            # Write only 'fields' to CSV
            header_fields, row_fields = extract_csv_data(
                response_get_full_form, "fields"
            )
            write_csv(
                header_fields,
                row_fields,
                os.path.join(
                    BASE_PATH, "4. answerAndQuestion", "questionMandatoryFields.csv"
                ),
            )

            # Write only 'additional_fields' to CSV
            header_add_fields, row_add_fields = extract_csv_data(
                response_get_full_form, "additional_fields"
            )
            write_csv(
                header_add_fields,
                row_add_fields,
                os.path.join(
                    BASE_PATH, "4. answerAndQuestion", "questionAdditionalFields.csv"
                ),
            )

        # Print summary of responses
        for status, count in response_counters.items():
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
