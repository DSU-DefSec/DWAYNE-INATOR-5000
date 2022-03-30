import time
from pyModbusTCP.client import ModbusClient
import sys

SERVER_HOST = str(sys.argv[1])
SERVER_PORT = 502
SERVER_U_ID = 1

c = ModbusClient()

c.host(SERVER_HOST)
c.port(SERVER_PORT)
c.unit_id(SERVER_U_ID)

if not c.is_open():
	if not c.open():
		print("No connct to "+SERVER_HOST+":"+str(SERVER_PORT))

if c.is_open():
	check = c.read_coils(103, 1)
	print(check[0])

