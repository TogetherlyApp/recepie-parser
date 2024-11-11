[![Apache V2 License](https://img.shields.io/badge/License-Apache%20v2-green.svg)](https://opensource.org/license/apache-2-0)

# Recepie-Parser

Recepie parser is a simple HTTP Service that exposes an Endpoint for any desired internet recepie and parses the ingredients using Gemini. 
It uses Supabase Auth JWT Tokens for authentication.


## Environment Variables

To run this project, you will need to add the following environment variables to your .env file

`SUPABASE_JWT_SECRET` JWT Secret of Supabase used to parse the JWT token.

`GOOGLE_AI_APIKEY` Gemini API Key

`HOST` Host for the webserver to listen on

`PORT` Port of the webserver to listen on