import requests
from flask import Flask

app = Flask(__name__)


@app.route("/")
def weather():
    url = "https://api.open-meteo.com/v1/forecast?latitude=41.31&longitude=69.27&current_weather=true"
    response = requests.get(url)
    return response.text


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
