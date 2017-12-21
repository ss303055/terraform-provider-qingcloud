resource "null_resource" "run_docker_wordpress" {
  depends_on = [
    "null_resource.run_docker_mysql",
  ]

  provisioner "remote-exec" {
    inline = [
      "curl -fsSL get.docker.com -o get-docker.sh",
      "sh get-docker.sh",
      "systemctl start docker",
      "docker run --name wordpress -d -p 80:80 -e WORDPRESS_DB_HOST=${var.mysql_instance_private_ip} -e WORDPRESS_DB_PASSWORD=wordpress wordpress",
    ]

    connection {
      type        = "ssh"
      user        = "root"
      host        = "${var.wordpress_instance_public_ip}"
      private_key = "${file("~/.ssh/id_rsa")}"
      port        = "${var.mysql_public_ssh_port}"
    }
  }
}

resource "null_resource" "run_docker_mysql" {
  provisioner "remote-exec" {
    inline = [
      "curl -fsSL get.docker.com -o get-docker.sh",
      "sh get-docker.sh",
      "systemctl start docker",
      "docker run --name wordpress-mysql -v /datadir:/var/lib/mysql  -p 3306:3306 -e MYSQL_ROOT_PASSWORD=wordpress -d  mysql:5.7",
    ]

    connection {
      type        = "ssh"
      user        = "root"
      host        = "${var.mysql_instance_public_ip}"
      private_key = "${file("~/.ssh/id_rsa")}"
      port        = "${var.mysql_public_ssh_port}"
    }
  }
}
