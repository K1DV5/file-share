# -{cd ../../../.. | share}

from http.server import BaseHTTPRequestHandler, HTTPServer
from os import path, listdir, remove
from cgi import FieldStorage
from json import dumps
from mimetypes import types_map
from urllib.parse import unquote

index = path.join(path.dirname(__file__), 'index.html')
own_files = {}  # files here that came from the clients to grant delete access


class handler(BaseHTTPRequestHandler):

    def send_node(self, node):
        self.send_response(200)
        if path.isdir(node):
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            to_write = []
            own = own_files.get(self.client_address[0], [])
            for sub in listdir(node):
                fname = path.join(node, sub)
                if path.isdir(fname):
                    kind = 'folder'
                else:
                    kind = types_map.get(path.splitext(sub)[1])
                to_write.append({'type': kind, 'name': sub,
                                 'own': fname in own})
            self.wfile.write(dumps(to_write, ensure_ascii=False).encode())
            return
        self.send_header('Content-Type', types_map.get(path.splitext(node)[1]))
        with open(node, 'rb') as file:
            content = file.read()
            self.send_header('Content-Length', len(content))
        self.end_headers()
        self.wfile.write(content)

    def do_GET(self):
        # for files
        if self.path == '/':
            return self.send_node(index)
        elif self.path.startswith('/get/'):
            self.path = unquote(self.path[4:]).strip('/') \
                if self.path[5:] else '.'
        if path.exists(self.path):
            return self.send_node(self.path)
        self.send_response(404)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write(b'404: Not found')

    def do_POST(self):
        self.send_response(200)
        self.end_headers()
        env = {'REQUEST_METHOD': 'POST',
               'CONTENT-TYPE': self.headers['Content-Type']}
        form = FieldStorage(fp=self.rfile, headers=self.headers, environ=env)
        folder = form.getvalue('folder')[1:]
        files = form['file']
        if type(files) != list:  # single file
            files = [files]
        if self.client_address[0] not in own_files:
            own_files[self.client_address[0]] = set()
        inc = 50 / len(files)
        for i, file in enumerate(files, start=1):
            self.wfile.write(str(round(50 + i * inc)).encode())  # progress
            fname = path.join('.', folder, file.filename)
            own_files[self.client_address[0]].update([fname])
            with open(fname, 'wb') as fp:
                fp.write(file.file.read())

    def do_DELETE(self):
        fname = path.join('.', self.path[1:])
        if path.exists(fname):
            own = own_files.get(self.client_address[0], [])
            if fname in own:
                remove(fname)
                own.remove(fname)
                self.send_response(200)
            else:
                self.send_response(403)
        else:
            self.send_response(404)
        self.end_headers()


def serve():
    print('Serving...')
    HTTPServer(('', 80), handler).serve_forever()
