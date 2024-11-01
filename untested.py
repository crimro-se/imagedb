from flask import Flask, request, jsonify
from multiprocessing import Queue, Process, Manager
import base64
import time
import uuid
import torch
import multiprocessing as mp
from transformers import CLIPProcessor, CLIPModel
from PIL import Image
import io

def process_tasks(task_queue, results):
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32").to(device)
    processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")

    while True:
        task = task_queue.get()
        if task is None:
            # Stop signal
            break
        task_id, image_data, text_data = task
        try:
            embeddings = {}
            if image_data:
                image = Image.open(io.BytesIO(base64.b64decode(image_data)))
                inputs = processor(images=image, return_tensors="pt").to(device)
                with torch.no_grad():
                    image_embeddings = model.get_image_features(**inputs)
                embeddings['image'] = image_embeddings.cpu().numpy().tolist()
            if text_data:
                inputs = processor(text=text_data, return_tensors="pt").to(device)
                with torch.no_grad():
                    text_embeddings = model.get_text_features(**inputs)
                embeddings['text'] = text_embeddings.cpu().numpy().tolist()
            results[task_id] = embeddings
            print(embeddings)
        except Exception as e:
            results[task_id] = {'error': str(e)}

if __name__ == '__main__':
    mp.set_start_method('spawn')
    # Start the task processor in the background

    app = Flask(__name__)
    # Shared Queue for Tasks
    task_queue = Queue()

    # Shared Dictionary for Results (simplified for demo; not suitable for large-scale production)
    mgr = Manager() 
    results = mgr.dict()

    processor_process = Process(target=process_tasks, args=(task_queue,results))
    processor_process.daemon = True  # So it exits when main process exits
    processor_process.start()

    @app.route('/process', methods=['POST'])
    def process_endpoint():
        data = request.get_json()
        if not data:
            return jsonify({'error': 'No JSON data received'}), 400
        
        task_id = data.get('id') or str(uuid.uuid4())
        image_data = data.get('image')
        text_data = data.get('text')
        
        if not (image_data or text_data):
            return jsonify({'error': 'Either image or text data is required'}), 400
        
        task_queue.put((task_id, image_data, text_data))
        return jsonify({'message': 'Task submitted successfully', 'id': task_id}), 202

    @app.route('/result/<string:id>', methods=['GET'])
    def result_endpoint(id):
        if id not in results:
            return jsonify({'error': 'Result not found or not yet processed'}), 404
        return jsonify({'id': id, 'result': results[id]}), 200
    try:
        app.run(debug=False)
    finally:
        task_queue.put(None)  # Stop signal
        processor_process.join()